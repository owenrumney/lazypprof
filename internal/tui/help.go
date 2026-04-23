package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	helpKeyStyle   = lipgloss.NewStyle().Bold(true).Width(16)
	helpDescStyle  = lipgloss.NewStyle().Foreground(subtleColor)
)

func (m Model) helpView() string {
	var b strings.Builder

	section := func(title string, keys []helpEntry) {
		b.WriteString(helpTitleStyle.Render(title))
		b.WriteByte('\n')
		for _, k := range keys {
			b.WriteString("  ")
			b.WriteString(helpKeyStyle.Render(k.key))
			b.WriteString(helpDescStyle.Render(k.desc))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	globalKeys := []helpEntry{
		{"tab", "Cycle views"},
		{"s", "Cycle sample type"},
		{"/", "Filter by function name (regex)"},
		{"esc", "Clear filter"},
		{"?", "Toggle this help"},
		{"q / ctrl+c", "Quit"},
	}
	if m.liveConfig != nil {
		globalKeys = append([]helpEntry{
			{"m", "Switch profile type (cpu/heap/allocs/goroutine)"},
			{"p", "Pause/resume live refresh"},
		}, globalKeys...)
	}
	section("Global", globalKeys)

	switch m.view {
	case viewTop:
		section("Top View", []helpEntry{
			{"j/k ↑/↓", "Navigate rows"},
		})
	case viewTree:
		section("Tree View", []helpEntry{
			{"j/k ↑/↓", "Navigate"},
			{"l/→/enter", "Expand node"},
			{"h/←", "Collapse / go to parent"},
			{"space", "Toggle expand/collapse"},
			{"*", "Expand subtree"},
			{"0", "Collapse all"},
		})
	case viewFlame:
		section("Flame Graph", []helpEntry{
			{"h/j/k/l ←↓↑→", "Navigate frames"},
			{"enter", "Zoom into frame"},
			{"backspace", "Zoom out"},
			{"0", "Reset zoom"},
		})
	case viewGoroutine:
		section("Goroutines", []helpEntry{
			{"j/k ↑/↓", "Navigate groups"},
			{"g", "Cycle state filter"},
			{"enter", "Drill into state (unique stacks)"},
			{"backspace", "Back to state groups"},
		})
	}

	section("Filter Bar (when active)", []helpEntry{
		{"enter", "Apply filter"},
		{"esc", "Cancel"},
	})

	return b.String()
}

type helpEntry struct {
	key  string
	desc string
}
