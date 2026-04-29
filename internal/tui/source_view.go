package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

type sourceModel struct {
	prof     *profile.Profile
	function string
	lines    []profile.LineStat
	width    int
	height   int
	cursor   int
	offset   int
	src      map[string][]string
}

func newSourceModel(p *profile.Profile, function string) sourceModel {
	m := sourceModel{
		prof:     p,
		function: function,
		lines:    p.SourceLines(function),
		src:      make(map[string][]string),
	}
	return m
}

func (m *sourceModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.clampScroll()
}

func (m *sourceModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.clampScroll()
	}
}

func (m *sourceModel) moveDown() {
	if m.cursor < len(m.lines)-1 {
		m.cursor++
		m.clampScroll()
	}
}

func (m *sourceModel) visibleHeight() int {
	h := m.height - 4
	if h < 1 {
		h = 20
	}
	return h
}

func (m *sourceModel) clampScroll() {
	if m.cursor >= len(m.lines) {
		m.cursor = max(0, len(m.lines)-1)
	}
	vis := m.visibleHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *sourceModel) view() string {
	if m.function == "" {
		return placeholderStyle.Render("\n  Select a function in Top and press enter to open source.\n")
	}
	if len(m.lines) == 0 {
		return placeholderStyle.Render("\n  No source line data for " + m.function + "\n")
	}

	width := m.width
	if width <= 0 {
		width = 100
	}
	funcWidth := width - 58
	if funcWidth < 16 {
		funcWidth = 16
	}

	var b strings.Builder
	title := fmt.Sprintf(" %s | [j/k] navigate", m.function)
	b.WriteString(goroutineHeaderStyle.Render(truncate(title, width)))
	b.WriteByte('\n')

	header := fmt.Sprintf(" %-12s %-12s %-8s %-*s %s", "Flat", "Cum", "Line", funcWidth, "Source", "File")
	b.WriteString(diffHdrStyle.Render(truncate(header, width)))
	b.WriteByte('\n')

	vis := m.visibleHeight()
	end := m.offset + vis
	if end > len(m.lines) {
		end = len(m.lines)
	}

	for i := m.offset; i < end; i++ {
		st := m.lines[i]
		sourceText := m.sourceText(st.File, st.Line)
		lineNo := "-"
		fileLine := st.File
		if st.Line > 0 {
			lineNo = fmt.Sprintf("%d", st.Line)
			fileLine = fmt.Sprintf("%s:%d", st.File, st.Line)
		}
		line := fmt.Sprintf(" %-12s %-12s %-8s %-*s %s",
			formatValue(st.Flat, m.prof.Unit()),
			formatValue(st.Cum, m.prof.Unit()),
			lineNo,
			funcWidth,
			truncate(sourceText, funcWidth),
			fileLine,
		)
		line = truncate(line, width)
		if i == m.cursor {
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

func (m *sourceModel) sourceText(path string, line int) string {
	if path == "" || line <= 0 {
		return ""
	}
	lines, ok := m.src[path]
	if !ok {
		lines = readSourceLines(path)
		m.src[path] = lines
	}
	if line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}

func readSourceLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
