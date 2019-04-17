package model

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/evergreen-ci/pail"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestUploadDownloadSpeeds(t *testing.T) {
	sizes := map[string]int{
		"32MB": 1024 * 1024 * 32,
	}

	for key, size := range sizes {
		log, err := makeLogLines(size, 1000)
		require.NoError(t, err)

		artifactJSON := &LogArtifact{Dir: "JSON"}
		start := time.Now()
		require.NoError(t, artifactJSON.UploadLogLinesJSON(log))
		end := time.Now()
		fmt.Printf("JSON Upload: %s %s\n", key, end.Sub(start))
		start = time.Now()
		downloadedLog, err := artifactJSON.DownloadLogJSON()
		require.NoError(t, err)
		end = time.Now()
		fmt.Printf("JSON Download: %s %s\n", key, end.Sub(start))

		fmt.Println(downloadedLog[0].Timestamp)
		fmt.Println("=========================================")

		artifactPrefix := &LogArtifact{Dir: "PREFIX"}
		start = time.Now()
		require.NoError(t, artifactPrefix.UploadLogLinesPrefix(log))
		end = time.Now()
		fmt.Printf("Prefix Upload: %s %s\n", key, end.Sub(start))
		start = time.Now()
		_, err = artifactPrefix.DownloadLog()
		require.NoError(t, err)
		end = time.Now()
		fmt.Printf("Preifx Download: %s %s\n", key, end.Sub(start))
	}
}

func TestS3Speed(t *testing.T) {
	opts := pail.S3Options{
		Name:   "pail-bucket-test",
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(opts)
	require.NoError(t, err)

	f, err := os.Create("s3_speed_test.txt")
	require.NoError(t, err)
	f.Close()

	// 32 MB
	size := 1024 * 1024 * 32
	require.NoError(t, timeUpload(size, 1, makeBuffs(size, 1), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 100), bucket))

	// 100MB
	size = 1024 * 1024 * 100
	require.NoError(t, timeUpload(size, 1, makeBuffs(size, 1), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 100), bucket))

	// 500MB
	size = 1024 * 1024 * 500
	require.NoError(t, timeUpload(size, 1, makeBuffs(size, 1), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 10), bucket))
	require.NoError(t, timeUpload(size, 4, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 8, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 16, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 32, makeBuffs(size, 100), bucket))
	require.NoError(t, timeUpload(size, 64, makeBuffs(size, 100), bucket))
}

func makeLogLines(size, lines int) ([][]interface{}, error) {
	log := [][]interface{}{}
	lineSize := size / lines
	lastLineSize := size % lines
	startTime := time.Now()

	for i := 0; i < lines-1; i++ {
		timestamp := startTime.Add(-1 * time.Duration(i) * time.Second).Unix()
		line := make([]byte, lineSize-1)
		_, err := rand.Read(line)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		line = append(line, '\n')

		log = append(log, []interface{}{float64(timestamp), string(line)})
	}
	timestamp := startTime.Add(-1 * time.Duration(lines) * time.Second).Unix()
	line := make([]byte, lastLineSize-1)
	_, err := rand.Read(line)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	log = append(log, []interface{}{float64(timestamp), string(line)})

	return log, nil
}

func makeBuffs(size int, chunks int) [][]byte {
	buffs := make([][]byte, chunks)
	for i := 0; i < len(buffs); i++ {
		buffs[i] = make([]byte, size/chunks)
		rand.Read(buffs[i])
	}
	return buffs
}

func timeUpload(size, threads int, buffs [][]byte, bucket pail.Bucket) error {
	var wg sync.WaitGroup
	guard := make(chan struct{}, threads)

	sizeDir := fmt.Sprintf("%dsize", size)
	chunksDir := fmt.Sprintf("%dchunks", len(buffs))
	dir := filepath.Join(sizeDir, chunksDir)
	start := time.Now()
	wg.Add(len(buffs))
	for i := 0; i < len(buffs); i++ {
		path := filepath.Join(dir, fmt.Sprintf("chunk_%d", i))
		buff := buffs[i]
		guard <- struct{}{}
		go func() {
			defer func() {
				wg.Done()
				<-guard
			}()
			if err := bucket.Put(context.TODO(), path, bytes.NewReader(buff)); err != nil {
				log.Fatal(err)
			}
		}()
	}
	wg.Wait()
	end := time.Now()

	f, err := os.OpenFile("s3_speed_test.txt", os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	info := "===================================\n"
	info += fmt.Sprintf("size: %d\n", size)
	info += fmt.Sprintf("chunks: %d\n", len(buffs))
	info += fmt.Sprintf("threads: %d\n", threads)
	info += fmt.Sprintf("time: %s\n", end.Sub(start))
	info += fmt.Sprintf("throughput: %f\n", float64(int64(size)*int64(time.Second)/int64(end.Sub(start))))
	info += "===================================\n"
	_, err = f.WriteString(info)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
