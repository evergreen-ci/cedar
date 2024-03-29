package model

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDownloadURL(t *testing.T) {
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_INTEGRATION_TESTS")); skip {
		t.Skip("SKIP_INTEGRATION_TESTS is set, skipping integration test with S3")
	}
	ctx := context.TODO()
	path := "download-test.txt"
	s3Name := "build-test-curator"
	s3Prefix := "perf-storage-test"
	s3Opts := pail.S3Options{
		Name:        s3Name,
		Region:      "us-east-1",
		Permissions: pail.S3PermissionsPublicRead,
	}
	s3BucketNoPrefix, err := pail.NewS3Bucket(s3Opts)
	require.NoError(t, err)
	s3Opts.Prefix = s3Prefix
	s3BucketPrefix, err := pail.NewS3Bucket(s3Opts)
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
			expectedURL: fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s3Name, s3Prefix+"/"+path),
			bucket:      s3BucketPrefix,
		},
		{
			name: "S3URLNoPrefix",
			artifact: ArtifactInfo{
				Type:   PailS3,
				Bucket: s3Name,
				Path:   path,
			},
			expectedURL: fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s3Name, path),
			bucket:      s3BucketNoPrefix,
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
				require.NoError(t, test.bucket.Put(ctx, path, strings.NewReader(expectedString)))
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
