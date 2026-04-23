package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_PushAndAt(t *testing.T) {
	h := NewHistory(3)

	assert.Equal(t, 0, h.Len())

	p1 := &Profile{SampleType: "one"}
	p2 := &Profile{SampleType: "two"}
	p3 := &Profile{SampleType: "three"}

	h.Push(p1)
	h.Push(p2)
	h.Push(p3)

	assert.Equal(t, 3, h.Len())
	assert.Equal(t, "one", h.At(0).Prof.SampleType)
	assert.Equal(t, "two", h.At(1).Prof.SampleType)
	assert.Equal(t, "three", h.At(2).Prof.SampleType)
}

func TestHistory_Wraparound(t *testing.T) {
	h := NewHistory(3)

	for i := range 5 {
		p := &Profile{SampleType: string(rune('a' + i))}
		h.Push(p)
	}

	// Should contain c, d, e (oldest two evicted).
	require.Equal(t, 3, h.Len())
	assert.Equal(t, "c", h.At(0).Prof.SampleType)
	assert.Equal(t, "d", h.At(1).Prof.SampleType)
	assert.Equal(t, "e", h.At(2).Prof.SampleType)
}

func TestHistory_Latest(t *testing.T) {
	h := NewHistory(5)

	p1 := &Profile{SampleType: "first"}
	p2 := &Profile{SampleType: "last"}

	h.Push(p1)
	h.Push(p2)

	assert.Equal(t, "last", h.Latest().Prof.SampleType)
}

func TestHistory_AtOutOfBounds(t *testing.T) {
	h := NewHistory(3)
	h.Push(&Profile{SampleType: "x"})

	assert.Equal(t, Snapshot{}, h.At(-1))
	assert.Equal(t, Snapshot{}, h.At(5))
}

func TestHistory_SnapshotHasTimestamp(t *testing.T) {
	h := NewHistory(3)
	h.Push(&Profile{SampleType: "timed"})

	snap := h.At(0)
	assert.False(t, snap.Time.IsZero())
}
