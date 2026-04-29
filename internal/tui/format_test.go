package tui

import "testing"

import "github.com/stretchr/testify/assert"

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name string
		v    int64
		unit string
		want string
	}{
		{"nanoseconds", 900, "nanoseconds", "900ns"},
		{"microseconds", 12_300, "nanoseconds", "12.30us"},
		{"milliseconds", 12_300_000, "nanoseconds", "12.30ms"},
		{"seconds", 1_230_000_000, "nanoseconds", "1.23s"},
		{"bytes", 512, "bytes", "512B"},
		{"kibibytes", 1536, "bytes", "1.50KiB"},
		{"count", 1200, "count", "1.20k"},
		{"negative", -1200, "count", "-1.20k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatValue(tt.v, tt.unit))
		})
	}
}
