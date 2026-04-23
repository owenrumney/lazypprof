package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

const topRowLimit = 200

func newTopTable(p *profile.Profile) table.Model {
	return buildTopTable(p, nil)
}

func newTopTableFiltered(p *profile.Profile, re *regexp.Regexp) table.Model {
	return buildTopTable(p, re)
}

func buildTopTable(p *profile.Profile, filter *regexp.Regexp) table.Model {
	stats := p.TopFunctions()
	if filter != nil {
		filtered := stats[:0:0]
		for _, s := range stats {
			if filter.MatchString(s.Name) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}
	if len(stats) > topRowLimit {
		stats = stats[:topRowLimit]
	}
	return buildNormalTable(stats)
}

func buildNormalTable(stats []profile.FunctionStat) table.Model {
	var total int64
	for _, s := range stats {
		total += s.Flat
	}
	if total == 0 {
		total = 1
	}

	rows := make([]table.Row, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", s.Flat),
			fmt.Sprintf("%.2f%%", 100*float64(s.Flat)/float64(total)),
			fmt.Sprintf("%d", s.Cum),
			fmt.Sprintf("%.2f%%", 100*float64(s.Cum)/float64(total)),
			truncate(s.Name, 80),
		})
	}

	cols := []table.Column{
		{Title: "Flat", Width: 12},
		{Title: "Flat%", Width: 8},
		{Title: "Cum", Width: 12},
		{Title: "Cum%", Width: 8},
		{Title: "Function", Width: 80},
	}

	return styledTable(cols, rows)
}

func styledTable(cols []table.Column, rows []table.Row) table.Model {
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtleColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(selectFg).
		Background(selectBg).
		Bold(false)
	t.SetStyles(s)

	return t
}

// diffTopView renders the diff top table with per-row coloring.
// The standard bubbles table doesn't support per-cell colors, so we render
// manually when in diff mode.
type diffTopView struct {
	stats  []profile.FunctionStat
	cursor int
	offset int
	width  int
	height int
}

var (
	diffPosStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"})   // red
	diffNegStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"})  // green
	diffHdrStyle = lipgloss.NewStyle().Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(subtleColor)
)

func newDiffTopView(p *profile.Profile, filter *regexp.Regexp) diffTopView {
	stats := p.TopFunctions()
	if filter != nil {
		filtered := stats[:0:0]
		for _, s := range stats {
			if filter.MatchString(s.Name) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}
	sort.Slice(stats, func(i, j int) bool {
		return abs64(stats[i].Cum) > abs64(stats[j].Cum)
	})
	if len(stats) > topRowLimit {
		stats = stats[:topRowLimit]
	}
	return diffTopView{stats: stats}
}

func (d *diffTopView) setSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *diffTopView) moveUp() {
	if d.cursor > 0 {
		d.cursor--
		d.clampScroll()
	}
}

func (d *diffTopView) moveDown() {
	if d.cursor < len(d.stats)-1 {
		d.cursor++
		d.clampScroll()
	}
}

func (d *diffTopView) visibleHeight() int {
	h := d.height - 3
	if h < 1 {
		h = 20
	}
	return h
}

func (d *diffTopView) clampScroll() {
	vis := d.visibleHeight()
	if d.cursor < d.offset {
		d.offset = d.cursor
	}
	if d.cursor >= d.offset+vis {
		d.offset = d.cursor - vis + 1
	}
}

func (d *diffTopView) view() string {
	if len(d.stats) == 0 {
		return placeholderStyle.Render("\n  No differences found\n")
	}

	funcWidth := d.width - 32
	if funcWidth < 20 {
		funcWidth = 20
	}

	var b strings.Builder

	header := fmt.Sprintf(" %-14s %-14s %s", "Flat Delta", "Cum Delta", "Function")
	b.WriteString(diffHdrStyle.Render(truncate(header, d.width)))
	b.WriteByte('\n')

	vis := d.visibleHeight()
	end := d.offset + vis
	if end > len(d.stats) {
		end = len(d.stats)
	}

	for i := d.offset; i < end; i++ {
		s := d.stats[i]
		flatStr := formatDelta(s.Flat)
		cumStr := formatDelta(s.Cum)
		name := truncate(s.Name, funcWidth)
		line := fmt.Sprintf(" %-14s %-14s %s", flatStr, cumStr, name)

		if i == d.cursor {
			line = padRight(line, d.width)
			b.WriteString(lipgloss.NewStyle().
				Background(selectBg).
				Foreground(selectFg).
				Bold(true).
				Render(line))
		} else if s.Cum > 0 {
			b.WriteString(diffPosStyle.Render(line))
		} else if s.Cum < 0 {
			b.WriteString(diffNegStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func formatDelta(v int64) string {
	if v > 0 {
		return fmt.Sprintf("+%d", v)
	}
	return fmt.Sprintf("%d", v)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}


func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}
