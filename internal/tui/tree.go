package tui

import (
	"fmt"
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

	var b strings.Builder
	for i := m.offset; i < end; i++ {
		r := m.rows[i]
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
		name := truncate(r.node.Func, funcWidth-len(indent))

		line := fmt.Sprintf("%s%s %8d %5.1f%%  %s",
			indent, marker, r.node.Cum, pct, name)

		if i == m.cursor {
			// Pad to full width so the highlight spans the row.
			if len(line) < m.width {
				line += strings.Repeat(" ", m.width-len(line))
			}
			line = treeCursorStyle.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
