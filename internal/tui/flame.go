package tui

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

// flameFrame is one rendered rectangle in the flame graph.
type flameFrame struct {
	node       *profile.Node
	x, width   int
	depth      int
	label      string
	colorIndex int
}

// flameModel holds the state for the flame graph view.
type flameModel struct {
	roots    []*profile.Node
	total    int64
	width    int
	height   int
	frames   []flameFrame // all laid-out frames
	cursor   int          // index into frames
	zoomRoot *profile.Node
	zoomHist []*profile.Node // stack for backspace
}

func newFlameModel(p *profile.Profile) flameModel {
	roots := p.CallGraph(0)
	total := p.TotalValue()
	m := flameModel{
		roots: roots,
		total: total,
	}
	return m
}

func (m *flameModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.layout()
}

func (m *flameModel) layout() {
	m.frames = m.frames[:0]
	if m.width <= 0 {
		return
	}

	var nodes []*profile.Node
	var rootTotal int64
	if m.zoomRoot != nil {
		nodes = []*profile.Node{m.zoomRoot}
		rootTotal = m.zoomRoot.Cum
	} else {
		nodes = m.roots
		rootTotal = m.total
	}
	if rootTotal <= 0 {
		rootTotal = 1
	}

	m.layoutChildren(nodes, 0, 0, m.width, rootTotal)
	m.clampCursor()
}

func (m *flameModel) layoutChildren(nodes []*profile.Node, x, depth, totalWidth int, rootTotal int64) {
	cx := x
	for _, n := range nodes {
		w := int(float64(totalWidth) * float64(n.Cum) / float64(rootTotal))
		if w < 1 {
			w = 1
		}
		// Don't overflow the allocated width.
		if cx+w > x+totalWidth {
			w = x + totalWidth - cx
		}
		if w <= 0 {
			continue
		}

		m.frames = append(m.frames, flameFrame{
			node:       n,
			x:          cx,
			width:      w,
			depth:      depth,
			label:      shortenFunc(n.Func, w),
			colorIndex: funcColor(n.Func),
		})

		m.layoutChildren(n.Children, cx, depth+1, w, rootTotal)
		cx += w
	}
}

// Navigation

func (m *flameModel) moveLeft() {
	if len(m.frames) == 0 {
		return
	}
	cur := m.frames[m.cursor]
	// Find closest frame at same depth to the left.
	best := -1
	for i, f := range m.frames {
		if f.depth == cur.depth && f.x+f.width <= cur.x {
			if best < 0 || f.x > m.frames[best].x {
				best = i
			}
		}
	}
	if best >= 0 {
		m.cursor = best
	}
}

func (m *flameModel) moveRight() {
	if len(m.frames) == 0 {
		return
	}
	cur := m.frames[m.cursor]
	best := -1
	for i, f := range m.frames {
		if f.depth == cur.depth && f.x >= cur.x+cur.width {
			if best < 0 || f.x < m.frames[best].x {
				best = i
			}
		}
	}
	if best >= 0 {
		m.cursor = best
	}
}

func (m *flameModel) moveUp() {
	if len(m.frames) == 0 {
		return
	}
	cur := m.frames[m.cursor]
	// Find frame one depth higher that overlaps our x range.
	mid := cur.x + cur.width/2
	for i, f := range m.frames {
		if f.depth == cur.depth-1 && f.x <= mid && f.x+f.width > mid {
			m.cursor = i
			return
		}
	}
}

func (m *flameModel) moveDown() {
	if len(m.frames) == 0 {
		return
	}
	cur := m.frames[m.cursor]
	mid := cur.x + cur.width/2
	best := -1
	bestDist := m.width
	for i, f := range m.frames {
		if f.depth == cur.depth+1 && f.x < cur.x+cur.width && f.x+f.width > cur.x {
			fmid := f.x + f.width/2
			dist := mid - fmid
			if dist < 0 {
				dist = -dist
			}
			if dist < bestDist {
				bestDist = dist
				best = i
			}
		}
	}
	if best >= 0 {
		m.cursor = best
	}
}

func (m *flameModel) zoomIn() {
	if len(m.frames) == 0 {
		return
	}
	f := m.frames[m.cursor]
	if len(f.node.Children) == 0 {
		return
	}
	m.zoomHist = append(m.zoomHist, m.zoomRoot)
	m.zoomRoot = f.node
	m.layout()
}

func (m *flameModel) zoomOut() {
	if len(m.zoomHist) == 0 {
		return
	}
	m.zoomRoot = m.zoomHist[len(m.zoomHist)-1]
	m.zoomHist = m.zoomHist[:len(m.zoomHist)-1]
	m.layout()
}

func (m *flameModel) zoomReset() {
	m.zoomRoot = nil
	m.zoomHist = nil
	m.layout()
}

func (m *flameModel) clampCursor() {
	if len(m.frames) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.frames) {
		m.cursor = len(m.frames) - 1
	}
}

// Rendering

// Flame palette using adaptive colors. Each entry picks from the terminal's
// base 16 colors so the graph looks reasonable in any theme.
var flameColors = []lipgloss.AdaptiveColor{
	{Light: "1", Dark: "9"},   // red / bright red
	{Light: "3", Dark: "11"},  // yellow / bright yellow
	{Light: "2", Dark: "10"},  // green / bright green
	{Light: "6", Dark: "14"},  // cyan / bright cyan
	{Light: "5", Dark: "13"},  // magenta / bright magenta
	{Light: "1", Dark: "11"},  // red / bright yellow
	{Light: "3", Dark: "9"},   // yellow / bright red
	{Light: "2", Dark: "13"},  // green / bright magenta
}

func funcColor(name string) int {
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % uint32(len(flameColors)))
}

func (m *flameModel) view() string {
	if len(m.frames) == 0 {
		return placeholderStyle.Render("\n  No flame graph data\n")
	}

	// Find the max depth to determine how many rows we need.
	maxDepth := 0
	for _, f := range m.frames {
		if f.depth > maxDepth {
			maxDepth = f.depth
		}
	}

	// Available rows for the graph (reserve 2 for status line).
	availRows := m.height - 2
	if availRows < 1 {
		availRows = 10
	}

	// If the graph is taller than the viewport, show the bottom (around cursor).
	startDepth := 0
	if maxDepth+1 > availRows {
		curDepth := 0
		if m.cursor < len(m.frames) {
			curDepth = m.frames[m.cursor].depth
		}
		// Center the cursor depth in the viewport.
		startDepth = curDepth - availRows/2
		if startDepth < 0 {
			startDepth = 0
		}
		if startDepth+availRows > maxDepth+1 {
			startDepth = maxDepth + 1 - availRows
		}
	}

	endDepth := startDepth + availRows

	// Build each row as a character buffer. Root at bottom (Brendan Gregg convention).
	rowCount := maxDepth + 1
	if rowCount > availRows {
		rowCount = availRows
	}

	type cell struct {
		ch    rune
		color int
		focus bool
	}

	grid := make([][]cell, rowCount)
	for i := range grid {
		row := make([]cell, m.width)
		for j := range row {
			row[j] = cell{ch: ' ', color: -1}
		}
		grid[i] = row
	}

	// Place frames. Root at bottom means depth 0 → last row.
	for fi, f := range m.frames {
		if f.depth < startDepth || f.depth >= endDepth {
			continue
		}
		// Map depth to grid row: root (depth=startDepth) at bottom.
		gridRow := rowCount - 1 - (f.depth - startDepth)
		if gridRow < 0 || gridRow >= rowCount {
			continue
		}
		isFocused := fi == m.cursor

		label := f.label
		labelRunes := []rune(label)

		for col := f.x; col < f.x+f.width && col < m.width; col++ {
			ci := col - f.x
			ch := ' '
			if ci < len(labelRunes) {
				ch = labelRunes[ci]
			}
			grid[gridRow][col] = cell{
				ch:    ch,
				color: f.colorIndex,
				focus: isFocused,
			}
		}
	}

	// Render grid to string.
	var b strings.Builder
	for _, row := range grid {
		col := 0
		for col < len(row) {
			// Find a run of cells with the same style.
			startCol := col
			c := row[col]
			for col < len(row) && row[col].color == c.color && row[col].focus == c.focus {
				col++
			}

			// Build the text for this run.
			var run strings.Builder
			for k := startCol; k < col; k++ {
				run.WriteRune(row[k].ch)
			}

			text := run.String()
			if c.color < 0 {
				b.WriteString(text)
			} else {
				style := lipgloss.NewStyle()
				if c.focus {
					style = style.Background(selectBg).
						Foreground(selectFg).
						Bold(true)
				} else {
					style = style.Background(flameColors[c.color]).
						Foreground(textColor)
				}
				b.WriteString(style.Render(text))
			}
		}
		b.WriteByte('\n')
	}

	// Status line: show focused frame info.
	if m.cursor < len(m.frames) {
		f := m.frames[m.cursor]
		pct := 100 * float64(f.node.Cum) / float64(m.total)
		status := fmt.Sprintf(" %s  Self: %d  Cum: %d (%.1f%%)  [enter] zoom  [backspace] back  [0] reset",
			f.node.Func, f.node.Self, f.node.Cum, pct)
		b.WriteString(lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Render(truncate(status, m.width)))
	}

	return b.String()
}

// shortenFunc produces a label that fits in width cells.
func shortenFunc(name string, width int) string {
	if width < 1 {
		return ""
	}
	if len(name) <= width {
		return name
	}

	// Try progressively shorter forms:
	// 1. Drop the package path, keep type.Method
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		short := name[idx+1:]
		if len(short) <= width {
			return short
		}
		name = short
	}

	// 2. Truncate with ellipsis
	if width <= 1 {
		return string(name[0])
	}
	return name[:width-1] + "…"
}
