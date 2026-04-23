package profile

import "time"

// Snapshot is a timestamped profile stored in the history ring buffer.
type Snapshot struct {
	Prof *Profile
	Time time.Time
}

// History is a fixed-capacity ring buffer of profile snapshots.
type History struct {
	buf  []Snapshot
	cap  int
	head int // next write position
	len  int // number of stored entries
}

// NewHistory creates a ring buffer that holds up to cap snapshots.
func NewHistory(cap int) *History {
	return &History{
		buf: make([]Snapshot, cap),
		cap: cap,
	}
}

// Push adds a profile snapshot. If full, the oldest entry is overwritten.
// Uses the profile's FetchedAt timestamp if set, otherwise falls back to now.
func (h *History) Push(p *Profile) {
	t := p.FetchedAt
	if t.IsZero() {
		t = time.Now()
	}
	h.buf[h.head] = Snapshot{Prof: p, Time: t}
	h.head = (h.head + 1) % h.cap
	if h.len < h.cap {
		h.len++
	}
}

// Len returns how many snapshots are stored.
func (h *History) Len() int {
	return h.len
}

// At returns the snapshot at logical index i (0 = oldest).
func (h *History) At(i int) Snapshot {
	if i < 0 || i >= h.len {
		return Snapshot{}
	}
	// oldest entry is at (head - len) mod cap
	start := (h.head - h.len + h.cap) % h.cap
	return h.buf[(start+i)%h.cap]
}

// Latest returns the most recent snapshot.
func (h *History) Latest() Snapshot {
	if h.len == 0 {
		return Snapshot{}
	}
	return h.At(h.len - 1)
}
