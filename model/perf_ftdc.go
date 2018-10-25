package model

import (
	"context"
	"io"

	"github.com/mongodb/ftdc"
	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/pkg/errors"
)

const defaultPointsPerChunk = 10 * 1000 // ten thousand

// DumpPerformanceSeries takes a stream of PerformancePoints, converts
// them to FTDC data and writes that data to the provided writer.
//
// FTDC makes it complicated to stream data directly from the input
// channel to the writer, and indeed, this implementation will not
// start writing to the output stream until the input stream is
// exhausted, but future work should allow us to avoid that detail.
func DumpPerformanceSeries(ctx context.Context, stream <-chan PerformancePoint, metadata interface{}, output io.Writer) error {
	collector := ftdc.NewBatchCollector(defaultPointsPerChunk)

	var (
		data []byte
		doc  *bson.Document
		err  error
	)

	if metadata != nil {
		data, err = bson.Marshal(metadata)
		if err != nil {
			return errors.Wrap(err, "problem reading metadata")
		}

		doc, err = bson.ReadDocument(data)
		if err != nil {
			return errors.Wrap(err, "problem building metadata document")
		}

		collector.SetMetadata(doc)
	}

conversion:
	for {
		select {
		case <-ctx.Done():
			return errors.New("operation canceled")
		case point, ok := <-stream:
			if !ok {
				break conversion
			}

			data, err = bson.Marshal(point)
			if err != nil {
				return errors.Wrap(err, "problem document")
			}

			doc, err = bson.ReadDocument(data)
			if err != nil {
				return errors.Wrap(err, "problem converting document")
			}

			if err = collector.Add(doc); err != nil {
				return errors.Wrap(err, "problem adding document to ftdc")
			}
		}
	}

	var payload []byte
	payload, err = collector.Resolve()
	if err != nil {
		return errors.Wrap(err, "problem dumping ftdc data")
	}

	n, err := output.Write(payload)
	if err != nil {
		return errors.Wrap(err, "problem writing data")
	}
	if n != len(payload) {
		return errors.New("data improperly flushed")
	}

	return nil
}
