package perf

import (
	"testing"
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type EDMFixture struct {
	Series   []float64 `json:"series"`
	Expected []int     `json:"expected"`
	MinSize  int       `json:"minSize"`
	Beta     float64   `json:"beta"`
	Degree   string    `json:"degree"`
}

func TestFindChangePointsEDM(t *testing.T) {
	for _, test := range []struct {
		Name       string
		IsSlow     bool
		ShouldSkip bool
	}{
		{
			Name:   "TestShortEDM",
			IsSlow: false,
		},
		{
			Name:   "TestMediumEDM",
			IsSlow: false,
		},
		{
			Name:   "TestLargeEDM",
			IsSlow: false,
		},
		{
			Name:   "TestHugeEDM",
			IsSlow: true,
		},
		{
			Name:       "TestHumungousEDM",
			IsSlow:     true,
			ShouldSkip: isEvergreen(),
		},
		{
			Name:       "TestHumungousPlusEDM",
			IsSlow:     true,
			ShouldSkip: isEvergreen(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			if testing.Short() && test.IsSlow {
				t.Skip("skipping slow test")
			}
			if test.ShouldSkip {
				t.Skip("skipping on demand")
			}
			// Create fixture with defaults
			fixture := &EDMFixture{MinSize: 15, Beta: 2.0, Degree: "Quadratic"}
			require.NoError(t, LoadFixture(test.Name, fixture))

			start := time.Now()
			cpd := NewEDMDetector(fixture.MinSize).(*edmDetector)
			changePointIndexes := cpd.eDivisiveWithMedians(fixture.Series)
			grip.Info(message.Fields{
				"algorithm":    "EDivisiveMedians",
				"elapsed_secs": time.Since(start).Seconds(),
				"num_series":   len(fixture.Series),
			})
			assert.Equal(t, fixture.Expected, changePointIndexes)
		})
	}
}
