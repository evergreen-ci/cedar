package pail

import (
	"context"
	"io"
)

// Bucket defines an interface for accessing a remote blob store, like
// S3. Should be generic enough to be implemented for GCP equivalent,
// or even a GridFS backed system (mostly just for kicks.)
//
// Other goals of this project are to allow us to have a single
// interface for interacting with blob storage, and allow us to fully
// move off of our legacy goamz package and stabalize all blob-storage
// operations across all projects.
//
// See, the following implemenations for previous approaches.
//
//   - https://github.com/evergreen-ci/evergreen/blob/master/thirdparty/s3.go
//   - https://github.com/mongodb/curator/tree/master/sthree
//
// Eventually we'll move this package to its own repository, but for
// now we can do development here.

type Bucket interface {
	Check(context.Context) error
	Configure(BucketInfo) error

	// Writer and Reader. It may make sense
	Writer(context.Context) (io.WriteCloser, error)
	Reader(context.Context) (io.ReadCloser, error)

	// Put and Get write simple byte streams (in the form of
	// io.Readers) to/from specfied keys
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)

	// Upload and Download write files from the local file
	// system to the specified key.
	Upload(context.Context, string, string) error
	Download(context.Context, string, string) error

	// Sync methods: these methods are the recursive, efficient
	// copy methods of files from s3 to the local file
	// system.
	Push(context.Context, string, string) error
	Pull(context.Context, string, string) error

	// Copy does a special copy operation that does not require
	// downloading a file.
	Copy(context.Context, string, string) error

	// List provides a way to iterator over the contents of a
	// bucket (for a given prefix.)
	List(context.Context, string) (BucketIterator, error)
}

type BucketInfo struct {
	// TODO: this will probably need better types and a bunch of
	// validation. The current

	Auth   string
	Region string
	Name   string
}

////////////////////////////////////////////////////////////////////////
//
// Iterator

// While iterators (typically) use channels internally, this is a
// fairly standard paradigm for iterating through resources, and is
// use heavily in the FTDC library (https://github.com/mongodb/ftdc)
// and bson (https://godoc.org/github.com/mongodb/mongo-go-driver/bson)
// libraries.

type BucketIterator interface {
	Next(context.Context) bool
	Err() error
	Item() *BucketItem
}

type BucketItem struct {
	Bucket  string
	KeyName string

	// TODO add other info?

	// QUESTION: does this need to be an interface to support
	// additional information?

	b Bucket
}

func (bi *BucketItem) Get(ctx context.Context) (io.ReadCloser, error) {
	return bi.b.Get(ctx, bi.KeyName)
}
