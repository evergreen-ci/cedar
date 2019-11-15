package perf

import (
	"testing"
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ChangePointsFixture struct {
	Series       []float64 `json:"series"`
	Expected     []int     `json:"expected"`
	PValue       float64   `json:"p"`
	Permutations int       `json:"permutations"`
}

type QFixture struct {
	Series   []float64 `json:"series"`
	Expected []float64 `json:"expected"`
}

type ExtractQFixture struct {
	QValues  []float64 `json:"qs"`
	Expected struct {
		Index int     `json:"index"`
		Value float64 `json:"value"`
	} `json:"expected"`
}

func TestEdevisive(t *testing.T) {
	cpd := NewQHatDetector(0.0, 0, defaultSeed).(*qhatDetector)

	t.Run("TestQHat", func(t *testing.T) {
		fixture := QFixture{}

		require.NoError(t, LoadFixture(t.Name(), &fixture))
		series := fixture.Series
		expected := fixture.Expected

		values := cpd.qHat(series)
		assert.Equal(t, expected, values)
	})
	t.Run("TestExtractQ", func(t *testing.T) {
		fixture := ExtractQFixture{}
		require.NoError(t, LoadFixture(t.Name(), &fixture))

		qValues := fixture.QValues
		expectedIndex := fixture.Expected.Index
		expectedValue := fixture.Expected.Value

		index, value := cpd.extractQ(qValues)
		assert.Equal(t, expectedValue, value)
		assert.Equal(t, expectedIndex, index)
	})
}

func TestChangePoints(t *testing.T) {
	for _, test := range []struct {
		Name       string
		IsSlow     bool
		ShouldSkip bool
	}{
		{
			Name:   "TestChangePoints",
			IsSlow: false,
		},
		{
			Name:   "TestShort",
			IsSlow: false,
		},
		{
			Name:   "TestMedium",
			IsSlow: false,
		},
		{
			Name:   "TestLarge",
			IsSlow: true,
		},
		{
			Name:       "TestHuge",
			IsSlow:     true,
			ShouldSkip: isEvergreen(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			if testing.Short() && test.IsSlow {
				t.Skip("skipping slow test")
			}
			if test.ShouldSkip {
				t.Skip("skipping")
			}

			// Create fixture with defaults
			fixture := &ChangePointsFixture{PValue: .05, Permutations: 100}
			require.NoError(t, LoadFixture(test.Name, fixture))

			start := time.Now()
			cpd := NewQHatDetector(fixture.PValue, fixture.Permutations, defaultSeed)
			changePoints, err := cpd.DetectChanges(fixture.Series)
			require.NoError(t, err)
			grip.Info(message.Fields{
				"algorithm":       "EDivisive",
				"elapsed_seconds": time.Since(start).Seconds(),
				"num_series":      len(fixture.Series),
			})
			changePointIndexes := make([]int, len(changePoints))
			for i, cp := range changePoints {
				changePointIndexes[i] = cp.Index
			}
			assert.Equal(t, fixture.Expected, changePointIndexes)
		})
	}
}
