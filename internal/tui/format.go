package tui

import "fmt"

func formatValue(v int64, unit string) string {
	if v < 0 {
		return "-" + formatValue(-v, unit)
	}
	switch unit {
	case "nanoseconds":
		return formatDurationValue(v)
	case "bytes":
		return formatBytes(v)
	default:
		return formatCount(v)
	}
}

func formatDurationValue(ns int64) string {
	switch {
	case ns >= 1_000_000_000:
		return fmt.Sprintf("%.2fs", float64(ns)/1_000_000_000)
	case ns >= 1_000_000:
		return fmt.Sprintf("%.2fms", float64(ns)/1_000_000)
	case ns >= 1_000:
		return fmt.Sprintf("%.2fus", float64(ns)/1_000)
	default:
		return fmt.Sprintf("%dns", ns)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	value := float64(b)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.2fPiB", value/unit)
}

func formatCount(v int64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(v)/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.2fk", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}
