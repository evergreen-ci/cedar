package perf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortedList(t *testing.T) {
	t.Run("Insert", func(t *testing.T) {
		f := &sortedList{}
		f.Insert(1.0, 2.0, 3.0)

		assert.Equal(t, f, &sortedList{1.0, 2.0, 3.0})

		f.Insert(2.0)
		assert.Equal(t, f, &sortedList{1.0, 2.0, 2.0, 3.0})

		f.Insert(0.0)
		assert.Equal(t, f, &sortedList{0.0, 1.0, 2.0, 2.0, 3.0})

		f.Insert(4.0)
		assert.Equal(t, f, &sortedList{0.0, 1.0, 2.0, 2.0, 3.0, 4.0})
	})
	t.Run("Remove", func(t *testing.T) {
		f := &sortedList{0.0, 1.0, 2.0, 2.0, 3.0, 4.0}
		f.Remove(0.0)

		assert.Equal(t, f, &sortedList{1.0, 2.0, 2.0, 3.0, 4.0})

		f.Remove(2.0)
		assert.Equal(t, f, &sortedList{1.0, 2.0, 3.0, 4.0})

		f.Remove(0.0)
		assert.Equal(t, f, &sortedList{1.0, 2.0, 3.0, 4.0})

		f.Remove(4.0)
		assert.Equal(t, f, &sortedList{1.0, 2.0, 3.0})
	})
}
