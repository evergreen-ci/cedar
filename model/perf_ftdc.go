package model

import (
	"context"
	"io"

	"github.com/mongodb/ftdc"
	"github.com/mongodb/ftdc/events"
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
func DumpPerformanceSeries(ctx context.Context, stream <-chan events.Performance, metadata interface{}, output io.Writer) error {
	collector := ftdc.NewBatchCollector(defaultPointsPerChunk)

	if metadata != nil {
		if err := collector.SetMetadata(metadata); err != nil {
			return errors.WithStack(err)
		}
	}

conversion:
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "operation canceled")
		case point, ok := <-stream:
			if !ok {
				break conversion
			}

			if err := collector.Add(point); err != nil {
				return errors.Wrap(err, "adding document to FTDC")
			}
		}
	}

	payload, err := collector.Resolve()
	if err != nil {
		return errors.Wrap(err, "dumping FTDC data")
	}

	n, err := output.Write(payload)
	if err != nil {
		return errors.Wrap(err, "writing data")
	}
	if n != len(payload) {
		return errors.New("data improperly flushed")
	}

	return nil
}
