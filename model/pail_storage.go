package model

import (
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/pkg/errors"
)

type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS PailType = "gridfs-legacy"
	PailGridFS       PailType = "gridfs"
	PailLocal        PailType = "local"

	defaultS3Region = "us-east-1"
)

func (t PailType) Create(env cedar.Environment, bucket, prefix, permissions string) (pail.Bucket, error) {
	var b pail.Bucket
	var err error
	ctx, cancel := env.Context()
	defer cancel()

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
			Credentials: pail.CreateAWSCredentials(conf.BucketCreds.AWSKey, conf.BucketCreds.AWSSecret, ""),
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
			Prefix:   bucket,
		}
		b, err = pail.NewGridFSBucketWithClient(ctx, client, opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case PailLocal:
		opts := pail.LocalOptions{
			Path: bucket,
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

func (t PailType) GetDownloadURL(bucket, prefix, path string) string {
	switch t {
	case PailS3:
		return fmt.Sprintf(
			"https://%s.s3.amazonaws.com/%s",
			bucket,
			prefix+"/"+path,
		)
	default:
		return ""
	}
}
