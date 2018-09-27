package pail

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
)

type s3Bucket struct {
	name string
	sess *session.Session
	svc  *s3.S3
}

func News3Bucket(s3BucketInfo BucketInfo) (Bucket, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(s3BucketInfo.Region)})
	if err != nil {
		return &s3Bucket{}, errors.Wrap(err, "problem connecting to AWS")
	}
	svc := s3.New(sess)
	return &s3Bucket{name: s3BucketInfo.Name, sess: sess, svc: svc}, nil
}

func (s *s3Bucket) String() string { return s.name }

func (s *s3Bucket) Check(ctx context.Context) error {
	input := &s3.GetBucketLocationInput{
		Bucket: aws.String(s.name),
	}

	result, err := s.svc.GetBucketLocationWithContext(ctx, input)
	if err != nil {
		return errors.Wrap(err, "problem getting bucket location")
	}
	if *result.LocationConstraint != *s.svc.Client.Config.Region {
		return errors.New("bucket does not exist in given region.")
	}
	return nil
}

type s3WriteCloser struct {
	buffer         []byte
	maxSize        int
	isCreated      bool
	isClosed       bool
	name           string
	svc            *s3.S3
	ctx            context.Context
	key            string
	partNumber     int64
	uploadId       string
	completedParts []*s3.CompletedPart
}

func (w *s3WriteCloser) create() error {
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(w.name),
		Key:    aws.String(w.key),
	}

	result, err := w.svc.CreateMultipartUploadWithContext(w.ctx, input)
	if err != nil {
		return errors.Wrap(err, "problem creating a multipart upload")
	}
	w.isCreated = true
	w.uploadId = *result.UploadId
	w.partNumber++
	return nil
}

func (w *s3WriteCloser) complete() error {
	input := &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(w.name),
		Key:    aws.String(w.key),
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: w.completedParts,
		},
		UploadId: aws.String(w.uploadId),
	}

	_, err := w.svc.CompleteMultipartUploadWithContext(w.ctx, input)
	if err != nil {
		w.abort()
		return errors.Wrap(err, "problem completing multipart upload")
	}
	return nil
}

func (w *s3WriteCloser) abort() {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(w.name),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadId),
	}

	w.svc.AbortMultipartUploadWithContext(w.ctx, input)
}

func (w *s3WriteCloser) flush() error {
	if !w.isCreated {
		err := w.create()
		if err != nil {
			return err
		}
	}
	input := &s3.UploadPartInput{
		Body:       aws.ReadSeekCloser(strings.NewReader(string(w.buffer))),
		Bucket:     aws.String(w.name),
		Key:        aws.String(w.key),
		PartNumber: aws.Int64(w.partNumber),
		UploadId:   aws.String(w.uploadId),
	}
	result, err := w.svc.UploadPartWithContext(w.ctx, input)
	if err != nil {
		w.abort()
		return errors.Wrap(err, "problem uploading part")
	}
	fmt.Println(w.partNumber)
	w.completedParts = append(w.completedParts, &s3.CompletedPart{
		ETag:       result.ETag,
		PartNumber: aws.Int64(w.partNumber),
	})
	w.partNumber++
	return nil
}

func (w *s3WriteCloser) Write(p []byte) (int, error) {
	if w.isClosed {
		errors.New("writer already closed!")
	}
	if len(w.buffer)+len(p) > w.maxSize {
		err := w.flush()
		if err != nil {
			return 0, err
		}
	}
	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *s3WriteCloser) Close() error {
	if w.isClosed {
		return errors.New("writer already closed!")
	}
	if len(w.buffer) > 0 {
		err := w.flush()
		if err != nil {
			return err
		}
	}
	err := w.complete()
	if err != nil {
		return err
	}
	return nil
}

func (s *s3Bucket) Writer(ctx context.Context, key string) (io.WriteCloser, error) {
	// 5MB is the minimum size for a multipart upload, so buffer needs to be at least that big.
	return &s3WriteCloser{
		maxSize: 5000000,
		name:    s.name,
		svc:     s.svc,
		ctx:     ctx,
		key:     key,
	}, nil
}

func (s *s3Bucket) Reader(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.name),
		Key:    aws.String(key),
	}

	result, err := s.svc.GetObjectWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (s *s3Bucket) Put(ctx context.Context, key string, r io.Reader) error {
	input := &s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(r),
		Bucket: aws.String(s.name),
		Key:    aws.String(key),
	}

	_, err := s.svc.PutObjectWithContext(ctx, input)
	return errors.Wrap(err, "problem copying data to file")
}

func (s *s3Bucket) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.Reader(ctx, key)
}

func (s *s3Bucket) Upload(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "problem opening file %s", path)
	}
	defer f.Close()

	return errors.WithStack(s.Put(ctx, key, f))
}

func (s *s3Bucket) Download(ctx context.Context, key, path string) error {
	reader, err := s.Reader(ctx, key)
	if err != nil {
		return errors.WithStack(err)
	}

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.Wrapf(err, "problem creating enclosing directory for '%s'", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "problem creating file '%s'", path)
	}
	_, err = io.Copy(f, reader)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "problem copying data")
	}

	return errors.WithStack(f.Close())
}

func (s *s3Bucket) Push(ctx context.Context, local, remote string) error {
	files, err := walkLocalTree(ctx, local)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, fn := range files {
		target := filepath.Join(remote, fn)
		file := filepath.Join(local, fn)
		localmd5, err := md5sum(file)
		if err != nil {
			return errors.Wrapf(err, "problem checksumming '%s'", file)
		}
		input := &s3.HeadObjectInput{
			Bucket:  aws.String(s.name),
			Key:     aws.String(target),
			IfMatch: aws.String(localmd5),
		}
		_, err = s.svc.HeadObjectWithContext(ctx, input)
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "PreconditionFailed" || aerr.Code() == "NotFound" {
				if err = s.Upload(ctx, target, file); err != nil {
					return errors.Wrapf(err, "problem uploading '%s' to '%s'",
						file, target)
				}
			}
		} else if err != nil {
			return errors.Wrapf(err, "problem finding '%s'", target)
		}
	}
	return nil
}

func (s *s3Bucket) Pull(ctx context.Context, local, remote string) error {
	iter, err := s.List(ctx, remote)
	if err != nil {
		return errors.WithStack(err)
	}

	for iter.Next(ctx) {
		if iter.Err() != nil {
			return errors.Wrap(err, "problem iterating bucket")
		}
		name, err := filepath.Rel(remote, iter.Item().Name())
		if err != nil {
			return errors.Wrap(err, "problem getting relative filepath")
		}
		localName := filepath.Join(local, name)
		localmd5, err := md5sum(localName)
		if os.IsNotExist(errors.Cause(err)) {
			if err = s.Download(ctx, iter.Item().Name(), localName); err != nil {
				return errors.WithStack(err)
			}
		} else if err != nil {
			return errors.WithStack(err)
		}
		if localmd5 != iter.Item().Hash() {
			if err = s.Download(ctx, iter.Item().Name(), localName); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (s *s3Bucket) Copy(ctx context.Context, src, dest string) error {
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.name),
		CopySource: aws.String(src),
		Key:        aws.String(dest),
	}

	_, err := s.svc.CopyObjectWithContext(ctx, input)
	if err != nil {
		return errors.Wrap(err, "problem copying data")
	}

	return nil
}

func (s *s3Bucket) Remove(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.name),
		Key:    aws.String(key),
	}

	_, err := s.svc.DeleteObjectWithContext(ctx, input)
	if err != nil {
		return errors.Wrap(err, "problem removing data")
	}
	return nil
}

func (s *s3Bucket) List(ctx context.Context, prefix string) (BucketIterator, error) {
	contents, isTruncated, err := s.getObjectsWrapper(ctx, prefix)
	if err != nil {
		return nil, err
	}
	return &s3BucketIterator{
		contents:    contents,
		idx:         -1,
		isTruncated: isTruncated,
		bucket:      s,
	}, nil
}

func (s *s3Bucket) getObjectsWrapper(ctx context.Context, prefix string) ([]*s3.Object, bool,
	error) {
	input := &s3.ListObjectsInput{
		Bucket: aws.String(s.name),
		Prefix: aws.String(prefix),
	}

	result, err := s.svc.ListObjectsWithContext(ctx, input)
	if err != nil {
		return nil, false, errors.Wrap(err, "problem listing objects")
	}
	return result.Contents, *result.IsTruncated, nil
}

type s3BucketIterator struct {
	contents    []*s3.Object
	idx         int
	isTruncated bool
	err         error
	item        *bucketItemImpl
	bucket      *s3Bucket
}

func (iter *s3BucketIterator) Err() error { return iter.err }

func (iter *s3BucketIterator) Item() BucketItem { return iter.item }

func (iter *s3BucketIterator) Next(ctx context.Context) bool {
	iter.idx++
	if iter.idx > len(iter.contents)-1 {
		if iter.isTruncated {
			contents, isTruncated, err := iter.bucket.getObjectsWrapper(ctx,
				*iter.contents[iter.idx-1].Key)
			if err != nil {
				iter.err = err
				return false
			}
			iter.contents = contents
			iter.idx = 0
			iter.isTruncated = isTruncated
		} else {
			return false
		}
	}

	iter.item = &bucketItemImpl{
		bucket: iter.bucket.name,
		key:    *iter.contents[iter.idx].Key,
		hash:   *iter.contents[iter.idx].ETag,
		b:      iter.bucket,
	}
	return true
}
