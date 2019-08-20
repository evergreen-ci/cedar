package model

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/pkg/errors"
)

// PailType describes the name of the blob storage backing a pail Bucket
// implementation.
type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS PailType = "gridfs-legacy"
	PailGridFS       PailType = "gridfs"
	PailLocal        PailType = "local"

	defaultS3Region = "us-east-1"
)

// Create returns a pail Bucket backed by PailType.
func (t PailType) Create(ctx context.Context, env cedar.Environment, bucket, prefix, permissions string) (pail.Bucket, error) {
	var b pail.Bucket
	var err error

	switch t {
	case PailS3:
		conf := &CedarConfig{}
		conf.Setup(env)
		if err = conf.Find(); err != nil {
			return nil, errors.Wrap(err, "problem getting application configuration")
		}

		opts := pail.S3Options{
			Name:        bucket,
			Prefix:      prefix,
			Region:      defaultS3Region,
			Permissions: pail.S3Permissions(permissions),
			Credentials: pail.CreateAWSCredentials(conf.Bucket.AWSKey, conf.Bucket.AWSSecret, ""),
			MaxRetries:  10,
		}
		b, err = pail.NewS3Bucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case PailLegacyGridFS, PailGridFS:
		client := env.GetClient()
		conf := env.GetConf()

		opts := pail.GridFSOptions{
			Database: conf.DatabaseName,
			Name:     bucket,
			Prefix:   prefix,
		}
		b, err = pail.NewGridFSBucketWithClient(ctx, client, opts)
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

// GetDownloadURL returns, if applicable, the download URL for the object at
// the given bucket/prefix/key location.
func (t PailType) GetDownloadURL(bucket, prefix, key string) string {
	switch t {
	case PailS3:
		return fmt.Sprintf(
			"https://%s.s3.amazonaws.com/%s",
			bucket,
			prefix+"/"+key,
		)
	default:
		return ""
	}
}
