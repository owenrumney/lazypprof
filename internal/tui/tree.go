package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

const maxRoots = 25

// treeRow is one visible line in the flattened tree.
type treeRow struct {
	node     *profile.Node
	depth    int
	expanded bool
	hasKids  bool
}

// treeModel holds the state for the collapsible tree view.
type treeModel struct {
	roots  []*profile.Node
	rows   []treeRow
	cursor int
	total  int64

	// Track which nodes are expanded by identity (pointer).
	open map[*profile.Node]bool

	// Filter highlighting.
	filterRe *regexp.Regexp
	matched  map[*profile.Node]bool // nodes whose Func matches the filter
	onPath   map[*profile.Node]bool // ancestors of a matched node

	width  int
	height int
	offset int // scroll offset
}

func newTreeModel(p *profile.Profile) treeModel {
	roots := p.CallGraph(maxRoots)
	total := p.TotalValue()
	m := treeModel{
		roots: roots,
		total: total,
		open:  make(map[*profile.Node]bool),
	}
	// Expand roots by default.
	for _, r := range roots {
		m.open[r] = true
	}
	m.rebuild()
	return m
}

func (m *treeModel) rebuild() {
	m.rows = m.rows[:0]
	for _, r := range m.roots {
		m.walk(r, 0)
	}
}

func (m *treeModel) walk(n *profile.Node, depth int) {
	expanded := m.open[n]
	m.rows = append(m.rows, treeRow{
		node:     n,
		depth:    depth,
		expanded: expanded,
		hasKids:  len(n.Children) > 0,
	})
	if expanded {
		for _, c := range n.Children {
			m.walk(c, depth+1)
		}
	}
}

func (m *treeModel) toggle() {
	if m.cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.cursor]
	if !r.hasKids {
		return
	}
	if m.open[r.node] {
		delete(m.open, r.node)
	} else {
		m.open[r.node] = true
	}
	m.rebuild()
}

func (m *treeModel) expand() {
	if m.cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.cursor]
	if r.hasKids && !m.open[r.node] {
		m.open[r.node] = true
		m.rebuild()
	}
}

func (m *treeModel) collapse() {
	if m.cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.cursor]
	if m.open[r.node] {
		delete(m.open, r.node)
		m.rebuild()
		return
	}
	// Move cursor to parent.
	if r.depth > 0 {
		for i := m.cursor - 1; i >= 0; i-- {
			if m.rows[i].depth < r.depth {
				m.cursor = i
				m.clampScroll()
				return
			}
		}
	}
}

func (m *treeModel) collapseAll() {
	for k := range m.open {
		delete(m.open, k)
	}
	m.cursor = 0
	m.offset = 0
	m.rebuild()
}

func (m *treeModel) expandSubtree() {
	if m.cursor >= len(m.rows) {
		return
	}
	m.expandAll(m.rows[m.cursor].node)
	m.rebuild()
}

func (m *treeModel) expandAll(n *profile.Node) {
	if len(n.Children) > 0 {
		m.open[n] = true
		for _, c := range n.Children {
			m.expandAll(c)
		}
	}
}

// computeMatches walks the tree and marks nodes that match the filter regex,
// plus their ancestors (onPath) so we can auto-expand to them.
func (m *treeModel) computeMatches() {
	m.matched = nil
	m.onPath = nil
	if m.filterRe == nil {
		return
	}
	m.matched = make(map[*profile.Node]bool)
	m.onPath = make(map[*profile.Node]bool)
	for _, r := range m.roots {
		m.markMatches(r)
	}
}

func (m *treeModel) markMatches(n *profile.Node) bool {
	direct := m.filterRe.MatchString(n.Func)
	if direct {
		m.matched[n] = true
	}
	childMatch := false
	for _, c := range n.Children {
		if m.markMatches(c) {
			childMatch = true
		}
	}
	if childMatch {
		m.onPath[n] = true
	}
	return direct || childMatch
}

// expandToMatches auto-expands all paths leading to matched nodes.
func (m *treeModel) expandToMatches() {
	if m.filterRe == nil {
		return
	}
	for _, r := range m.roots {
		m.expandIfOnPath(r)
	}
	m.rebuild()
}

func (m *treeModel) expandIfOnPath(n *profile.Node) {
	if m.onPath[n] || m.matched[n] {
		m.open[n] = true
		for _, c := range n.Children {
			m.expandIfOnPath(c)
		}
	}
}

func (m *treeModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.clampScroll()
	}
}

func (m *treeModel) moveDown() {
	if m.cursor < len(m.rows)-1 {
		m.cursor++
		m.clampScroll()
	}
}

func (m *treeModel) visibleHeight() int {
	h := m.height - 2 // leave room for header/footer
	if h < 1 {
		h = 20
	}
	return h
}

func (m *treeModel) clampScroll() {
	vis := m.visibleHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *treeModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

var treeCursorStyle = lipgloss.NewStyle().
	Background(selectBg).
	Foreground(selectFg)

var treeMatchStyle = lipgloss.NewStyle().
	Foreground(accentColor).
	Bold(true)

var treeDimStyle = lipgloss.NewStyle().
	Foreground(subtleColor)

func (m *treeModel) view() string {
	if len(m.rows) == 0 {
		return placeholderStyle.Render("\n  No call graph data\n")
	}

	vis := m.visibleHeight()
	end := m.offset + vis
	if end > len(m.rows) {
		end = len(m.rows)
	}

	funcWidth := m.width - 30 // space for indent + cum + pct
	if funcWidth < 20 {
		funcWidth = 20
	}

	// First pass: build lines and find the longest one.
	type viewLine struct {
		text      string
		separator bool           // blank line before this row
		node      *profile.Node  // for filter highlighting
	}
	lines := make([]viewLine, 0, end-m.offset)
	maxLen := 0
	for i := m.offset; i < end; i++ {
		r := m.rows[i]

		sep := r.depth == 0 && i > m.offset

		indent := strings.Repeat("  ", r.depth)

		marker := " "
		if r.hasKids {
			if r.expanded {
				marker = "▼"
			} else {
				marker = "▶"
			}
		}

		pct := 100 * float64(r.node.Cum) / float64(m.total)
		name := truncate(r.node.Func, funcWidth-len([]rune(indent)))

		line := fmt.Sprintf("%s%s %8d %5.1f%%  %s",
			indent, marker, r.node.Cum, pct, name)

		if n := len([]rune(line)); n > maxLen {
			maxLen = n
		}
		lines = append(lines, viewLine{text: line, separator: sep, node: r.node})
	}

	// Second pass: render with cursor padded to the longest line.
	var b strings.Builder
	b.WriteByte('\n') // gap between the header bar and first tree row
	for i, vl := range lines {
		if vl.separator {
			b.WriteByte('\n')
		}

		line := vl.text
		if m.offset+i == m.cursor {
			if n := len([]rune(line)); n < maxLen {
				line += strings.Repeat(" ", maxLen-n)
			}
			line = treeCursorStyle.Render(line)
		} else if m.filterRe != nil {
			if m.matched[vl.node] {
				line = treeMatchStyle.Render(line)
			} else if !m.onPath[vl.node] {
				line = treeDimStyle.Render(line)
			}
		}

		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
