package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestConvertTestResults(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		bsonData, err := os.ReadFile(filepath.Join("testdata", "test_results.bson"))
		require.NoError(t, err)
		var expectedResults []model.TestResult
		require.NoError(t, bson.Marshal(bsonData, &expectedResults))

		out, err := exec.Command("convert-bson-to-parquet", "test-results", "--local", filepath.Join("testdata", "test_results.bson")).Output()
		require.NoError(t, err)
	})
}
