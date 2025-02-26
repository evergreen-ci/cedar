package model

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// PailType describes the name of the blob storage backing a pail Bucket
// implementation.
type PailType string

const (
	PailS3    PailType = "s3"
	PailLocal PailType = "local"

	defaultS3Region     = "us-east-1"
	defaultS3MaxRetries = 10
)

// Create returns a Pail Bucket backed by PailType.
func (t PailType) Create(ctx context.Context, env cedar.Environment, bucket, prefix, permissions string, compress bool) (pail.Bucket, error) {
	var b pail.Bucket
	var err error

	switch t {
	case PailS3:
		conf := &CedarConfig{}
		conf.Setup(env)
		if err = conf.Find(); err != nil {
			return nil, errors.Wrap(err, "getting application configuration")
		}

		opts := pail.S3Options{
			Name:        bucket,
			Prefix:      prefix,
			Region:      defaultS3Region,
			Permissions: pail.S3Permissions(permissions),
			Credentials: pail.CreateAWSCredentials(conf.Bucket.AWSKey, conf.Bucket.AWSSecret, ""),
			MaxRetries:  utility.ToIntPtr(defaultS3MaxRetries),
			Compress:    compress,
		}
		b, err = pail.NewS3Bucket(ctx, opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case PailLocal:
		opts := pail.LocalOptions{
			Path:   bucket,
			Prefix: prefix,
		}
		b, err = pail.NewLocalBucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	default:
		return nil, errors.New("not implemented")
	}

	if err = b.Check(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	return b, nil
}

// CreatePresto returns a Pail Bucket backed by PailType specifically for
// buckets in our Presto ecosystem.
func (t PailType) CreatePresto(ctx context.Context, env cedar.Environment, prefix, permissions string, compress bool) (pail.Bucket, error) {
	conf := &CedarConfig{}
	conf.Setup(env)
	if err := conf.Find(); err != nil {
		return nil, errors.Wrap(err, "getting application configuration")
	}

	switch t {
	case PailS3:
		opts := pail.S3Options{
			Name:          conf.Bucket.PrestoBucket,
			Prefix:        prefix,
			Region:        defaultS3Region,
			Permissions:   pail.S3Permissions(permissions),
			Credentials:   pail.CreateAWSCredentials(conf.Bucket.AWSKey, conf.Bucket.AWSSecret, ""),
			AssumeRoleARN: conf.Bucket.PrestoRoleARN,
			MaxRetries:    utility.ToIntPtr(defaultS3MaxRetries),
			Compress:      compress,
		}
		b, err := pail.NewS3Bucket(ctx, opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if err = b.Check(ctx); err != nil {
			return nil, errors.WithStack(err)
		}

		return b, nil
	default:
		return t.Create(ctx, env, conf.Bucket.PrestoBucket, prefix, permissions, compress)
	}
}

// GetDownloadURL returns, if applicable, the download URL for the object at
// the given bucket/prefix/key location.
func (t PailType) GetDownloadURL(bucket, prefix, key string) string {
	switch t {
	case PailS3:
		return fmt.Sprintf(
			"https://%s.s3.amazonaws.com/%s",
			bucket,
			strings.Replace(filepath.Join(prefix, key), "\\", "/", -1),
		)
	default:
		return ""
	}
}
