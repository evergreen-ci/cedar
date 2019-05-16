package model

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDownloadURL(t *testing.T) {
	ctx := context.TODO()
	path := "download-test.txt"
	//s3Name := "build-test-curator"
	s3Name := "pail-bucket-test"
	s3Prefix := "perf-storage-test"
	s3Opts := pail.S3Options{
		Name:       s3Name,
		Prefix:     s3Prefix,
		Region:     "us-east-1",
		Permission: "public-read",
	}
	s3Bucket, err := pail.NewS3Bucket(s3Opts)
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		artifact    ArtifactInfo
		expectedURL string
		bucket      pail.Bucket
	}{
		{
			name: "S3URL",
			artifact: ArtifactInfo{
				Type:   PailS3,
				Bucket: s3Name,
				Prefix: s3Prefix,
				Path:   path,
			},
			expectedURL: fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s3Name, filepath.Join(s3Prefix, path)),
			bucket:      s3Bucket,
		},
		{
			name: "LocalURL",
			artifact: ArtifactInfo{
				Type:   PailLocal,
				Bucket: s3Name,
				Prefix: s3Prefix,
				Path:   path,
			},
		},
		{
			name: "GridFSURL",
			artifact: ArtifactInfo{
				Type:   PailGridFS,
				Bucket: s3Name,
				Prefix: s3Prefix,
				Path:   path,
			},
		},
		{
			name: "LegacyGridFSURL",
			artifact: ArtifactInfo{
				Type:   PailLegacyGridFS,
				Bucket: s3Name,
				Prefix: s3Prefix,
				Path:   path,
			},
		},
		{
			name: "EmptyType",
			artifact: ArtifactInfo{
				Bucket: s3Name,
				Prefix: s3Prefix,
				Path:   path,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			url := test.artifact.GetDownloadURL()
			assert.Equal(t, test.expectedURL, url)

			if test.expectedURL != "" {
				expectedString := "testing"
				test.bucket.Put(ctx, path, strings.NewReader(expectedString))
				defer func() {
					assert.NoError(t, test.bucket.Remove(ctx, path))
				}()

				resp, err := http.Get(url)
				require.NoError(t, err)
				defer resp.Body.Close()
				data, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, err)

				assert.Equal(t, expectedString, string(data))
			}
		})
	}
}
