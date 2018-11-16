package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/ftdc"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

const (
	ten             = 10
	hundred         = ten * ten
	thousand        = ten * hundred
	hundredThousand = hundred * thousand
	million         = ten * hundredThousand

	// things that really should be command line args
	outputFn   = "perf_metrics.ftdc"
	totalOps   = million
	bucketSize = hundredThousand
)

func main() {
	point := model.PerformancePoint{}
	startAt := time.Now()
	file, err := os.Create(outputFn)
	grip.EmergencyFatal(err)

	collector := ftdc.NewStreamingCollector(bucketSize, file)
	defer func() {
		stat, err := os.Stat(outputFn)
		grip.EmergencyFatal(err)

		grip.Info(message.Fields{
			"dur_secs":    time.Since(startAt).Seconds(),
			"bucket_size": bucketSize,
			"t":           totalOps,
			"size_bytes":  stat.Size(),
		})
	}()
	defer func() { grip.EmergencyFatal(file.Close()) }()
	defer func() { ftdc.FlushCollector(collector, file) }()

	grip.Info(message.Fields{
		"n":            0,
		"t":            totalOps,
		"bucket":       bucketSize,
		"info.metrics": collector.Info().MetricsCount,
		"info.samples": collector.Info().SampleCount,
	})

	ts := time.Now()
	for i := int64(1); i <= totalOps; i++ {
		ts = ts.Add(100 * time.Millisecond)
		point.Timestamp = ts

		point.Counters.Number = i
		point.Counters.Operations += i + rand.Int63n(10)

		if i%hundred == 0 {
			point.Counters.Size += i + rand.Int63n(10)
		}

		dur := time.Millisecond * time.Duration(i-1+rand.Int63n(40))
		point.Timers.Duration += dur - time.Duration(i+rand.Int63n(20))
		point.Timers.Total += dur

		point.Guages.Workers = 1
		if i&hundredThousand == 0 {
			point.Guages.State++
		}

		grip.InfoWhen(i%int64(bucketSize) == 0,
			message.Fields{
				"n":            i,
				"info.metrics": collector.Info().MetricsCount,
				"info.samples": collector.Info().SampleCount,
			})

		err = collector.Add(point)
		grip.EmergencyFatal(err)
	}
}
