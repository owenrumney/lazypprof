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

type topSort int

const (
	topSortCum topSort = iota
	topSortFlat
	topSortName
)

func (s topSort) label() string {
	switch s {
	case topSortFlat:
		return "flat"
	case topSortName:
		return "name"
	default:
		return "cum"
	}
}

type topView struct {
	allStats []profile.FunctionStat
	stats    []profile.FunctionStat
	sortBy   topSort
	desc     bool
	cursor   int
	offset   int
	width    int
	height   int
	unit     string
}

func newTopView(p *profile.Profile, filter *regexp.Regexp) topView {
	v := topView{
		allStats: topStats(p, filter),
		sortBy:   topSortCum,
		desc:     true,
		unit:     p.Unit(),
	}
	v.sort()
	return v
}

func topStats(p *profile.Profile, filter *regexp.Regexp) []profile.FunctionStat {
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
	return stats
}

func (v *topView) setSize(w, h int) {
	v.width = w
	v.height = h
	v.clampScroll()
}

func (v *topView) moveUp() {
	if v.cursor > 0 {
		v.cursor--
		v.clampScroll()
	}
}

func (v *topView) moveDown() {
	if v.cursor < len(v.stats)-1 {
		v.cursor++
		v.clampScroll()
	}
}

func (v *topView) setSort(sortBy topSort) {
	if v.sortBy == sortBy {
		v.desc = !v.desc
	} else {
		v.sortBy = sortBy
		v.desc = sortBy != topSortName
	}
	v.sort()
	v.clampScroll()
}

func (v *topView) sort() {
	v.stats = append(v.stats[:0], v.allStats...)
	sort.SliceStable(v.stats, func(i, j int) bool {
		var cmp int
		switch v.sortBy {
		case topSortFlat:
			cmp = compareInt64(v.stats[i].Flat, v.stats[j].Flat)
		case topSortName:
			cmp = strings.Compare(v.stats[i].Name, v.stats[j].Name)
		default:
			cmp = compareInt64(v.stats[i].Cum, v.stats[j].Cum)
		}
		if cmp == 0 {
			cmp = strings.Compare(v.stats[i].Name, v.stats[j].Name)
		}
		if v.desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if len(v.stats) > topRowLimit {
		v.stats = v.stats[:topRowLimit]
	}
}

func (v *topView) visibleHeight() int {
	h := v.height - 3
	if h < 1 {
		h = 20
	}
	return h
}

func (v *topView) clampScroll() {
	if v.cursor >= len(v.stats) {
		v.cursor = max(0, len(v.stats)-1)
	}
	vis := v.visibleHeight()
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+vis {
		v.offset = v.cursor - vis + 1
	}
}

func (v *topView) selectedFunction() string {
	if v.cursor < 0 || v.cursor >= len(v.stats) {
		return ""
	}
	return v.stats[v.cursor].Name
}

func (v *topView) view() string {
	if len(v.stats) == 0 {
		return placeholderStyle.Render("\n  No functions found\n")
	}

	width := v.width
	if width <= 0 {
		width = 100
	}
	funcWidth := width - 50
	if funcWidth < 20 {
		funcWidth = 20
	}

	var total int64
	for _, s := range v.stats {
		total += s.Flat
	}
	if total == 0 {
		total = 1
	}

	var b strings.Builder
	dir := "desc"
	if !v.desc {
		dir = "asc"
	}
	header := fmt.Sprintf(" %-12s %-8s %-12s %-8s %-*s  sort: %s %s", "Flat", "Flat%", "Cum", "Cum%", funcWidth, "Function", v.sortBy.label(), dir)
	b.WriteString(diffHdrStyle.Render(truncate(header, width)))
	b.WriteByte('\n')

	vis := v.visibleHeight()
	end := v.offset + vis
	if end > len(v.stats) {
		end = len(v.stats)
	}

	for i := v.offset; i < end; i++ {
		s := v.stats[i]
		line := fmt.Sprintf(" %-12s %-8s %-12s %-8s %s",
			formatValue(s.Flat, v.unit),
			fmt.Sprintf("%.2f%%", 100*float64(s.Flat)/float64(total)),
			formatValue(s.Cum, v.unit),
			fmt.Sprintf("%.2f%%", 100*float64(s.Cum)/float64(total)),
			truncate(s.Name, funcWidth),
		)
		if i == v.cursor {
			line = padRight(line, width)
			b.WriteString(lipgloss.NewStyle().
				Background(selectBg).
				Foreground(selectFg).
				Bold(true).
				Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func newTopTable(p *profile.Profile) table.Model {
	return buildTopTable(p, nil)
}

func newTopTableFiltered(p *profile.Profile, re *regexp.Regexp) table.Model {
	return buildTopTable(p, re)
}

func buildTopTable(p *profile.Profile, filter *regexp.Regexp) table.Model {
	stats := topStats(p, filter)
	if len(stats) > topRowLimit {
		stats = stats[:topRowLimit]
	}
	return buildNormalTable(stats, p.Unit())
}

func buildNormalTable(stats []profile.FunctionStat, unit string) table.Model {
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
			formatValue(s.Flat, unit),
			fmt.Sprintf("%.2f%%", 100*float64(s.Flat)/float64(total)),
			formatValue(s.Cum, unit),
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
	allStats []profile.FunctionStat
	stats    []profile.FunctionStat
	sortBy   topSort
	desc     bool
	cursor   int
	offset   int
	width    int
	height   int
	unit     string
}

var (
	diffPosStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"})  // red
	diffNegStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"}) // green
	diffHdrStyle = lipgloss.NewStyle().Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(subtleColor)
)

func newDiffTopView(p *profile.Profile, filter *regexp.Regexp) diffTopView {
	d := diffTopView{allStats: topStats(p, filter), sortBy: topSortCum, desc: true, unit: p.Unit()}
	d.sort()
	return d
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

func (d *diffTopView) setSort(sortBy topSort) {
	if d.sortBy == sortBy {
		d.desc = !d.desc
	} else {
		d.sortBy = sortBy
		d.desc = sortBy != topSortName
	}
	d.sort()
	d.clampScroll()
}

func (d *diffTopView) sort() {
	d.stats = append(d.stats[:0], d.allStats...)
	sort.SliceStable(d.stats, func(i, j int) bool {
		var cmp int
		switch d.sortBy {
		case topSortFlat:
			cmp = compareInt64(abs64(d.stats[i].Flat), abs64(d.stats[j].Flat))
		case topSortName:
			cmp = strings.Compare(d.stats[i].Name, d.stats[j].Name)
		default:
			cmp = compareInt64(abs64(d.stats[i].Cum), abs64(d.stats[j].Cum))
		}
		if cmp == 0 {
			cmp = strings.Compare(d.stats[i].Name, d.stats[j].Name)
		}
		if d.desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if len(d.stats) > topRowLimit {
		d.stats = d.stats[:topRowLimit]
	}
}

func (d *diffTopView) selectedFunction() string {
	if d.cursor < 0 || d.cursor >= len(d.stats) {
		return ""
	}
	return d.stats[d.cursor].Name
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

	dir := "desc"
	if !d.desc {
		dir = "asc"
	}
	header := fmt.Sprintf(" %-14s %-14s %s  sort: %s %s", "Flat Delta", "Cum Delta", "Function", d.sortBy.label(), dir)
	b.WriteString(diffHdrStyle.Render(truncate(header, d.width)))
	b.WriteByte('\n')

	vis := d.visibleHeight()
	end := d.offset + vis
	if end > len(d.stats) {
		end = len(d.stats)
	}

	for i := d.offset; i < end; i++ {
		s := d.stats[i]
		flatStr := formatDelta(s.Flat, d.unit)
		cumStr := formatDelta(s.Cum, d.unit)
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

func formatDelta(v int64, unit string) string {
	if v > 0 {
		return "+" + formatValue(v, unit)
	}
	if v < 0 {
		return "-" + formatValue(-v, unit)
	}
	return formatValue(v, unit)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
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
