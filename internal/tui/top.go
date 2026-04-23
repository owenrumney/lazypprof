package tui

import (
	"fmt"
	"regexp"

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

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
