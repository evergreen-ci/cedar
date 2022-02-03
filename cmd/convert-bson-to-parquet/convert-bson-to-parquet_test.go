package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
	"go.mongodb.org/mongo-driver/bson"
)

func TestConvertTestResults(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		bsonData, err := os.ReadFile(filepath.Join("testdata", "test_results.bson"))
		require.NoError(t, err)
		var bsonResults testResultsDoc
		require.NoError(t, bson.Unmarshal(bsonData, &bsonResults))
		expectedResults := make([]model.ParquetTestResult, len(bsonResults.Results))
		for i, result := range bsonResults.Results {
			expectedResults[i] = result.ConvertToParquetTestResult()
		}

		pfn := filepath.Join("testdata", "test_results.parquet")
		defer func() {
			assert.NoError(t, os.Remove(pfn))
		}()
		require.NoError(t, exec.Command("convert-bson-to-parquet", "test-results", "--local", filepath.Join("testdata", "test_results.bson"), "--out", pfn).Run())

		fr, err := local.NewLocalFileReader(pfn)
		require.NoError(t, err)
		pr, err := reader.NewParquetReader(fr, new(model.ParquetTestResult), 4)
		require.NoError(t, err)

		parquetResults := make([]model.ParquetTestResult, pr.GetNumRows())
		require.NoError(t, pr.Read(&parquetResults))
		pr.ReadStop()
		assert.Equal(t, expectedResults, parquetResults)
	})
}
