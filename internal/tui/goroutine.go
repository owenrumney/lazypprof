package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

// goroutineModel displays goroutines grouped by state, with drill-down into
// unique stacks within a state group.
type goroutineModel struct {
	goroutines []profile.Goroutine
	groups     []profile.GoroutineGroup // state groups (filtered)
	allStates  []string
	stateIdx   int // -1 = show all

	// Drill-down state.
	drilled     bool                 // true = viewing stacks within a group
	stackGroups []profile.StackGroup // unique stacks in the drilled group
	drillState  string               // which state we drilled into
	stackCursor int
	stackOffset int

	cursor int
	offset int
	width  int
	height int
}

func newGoroutineModel(p *profile.Profile) goroutineModel {
	m := goroutineModel{
		goroutines: p.Goroutines,
		allStates:  profile.GoroutineStates(p.Goroutines),
		stateIdx:   -1,
	}
	m.rebuildGroups()
	return m
}

func (m *goroutineModel) rebuildGroups() {
	if m.stateIdx >= len(m.allStates) {
		m.stateIdx = -1 // state no longer present after refresh
	}
	if m.stateIdx < 0 {
		m.groups = profile.GroupByState(m.goroutines)
	} else {
		state := m.allStates[m.stateIdx]
		var filtered []profile.Goroutine
		for _, g := range m.goroutines {
			if g.State == state {
				filtered = append(filtered, g)
			}
		}
		m.groups = profile.GroupByState(filtered)
	}
	if m.cursor >= len(m.groups) {
		m.cursor = max(0, len(m.groups)-1)
	}
	m.clampScroll()
}

func (m *goroutineModel) cycleState() {
	if m.drilled {
		return // don't change filter while drilled in
	}
	m.stateIdx++
	if m.stateIdx >= len(m.allStates) {
		m.stateIdx = -1
	}
	m.rebuildGroups()
}

func (m *goroutineModel) enter() {
	if m.drilled || m.cursor >= len(m.groups) {
		return
	}
	g := m.groups[m.cursor]
	m.drilled = true
	m.drillState = g.State
	m.stackGroups = profile.GroupByStack(g.Goroutines)
	m.stackCursor = 0
	m.stackOffset = 0
}

func (m *goroutineModel) back() {
	if !m.drilled {
		return
	}
	m.drilled = false
	m.stackGroups = nil
	m.drillState = ""
}

func (m *goroutineModel) moveUp() {
	if m.drilled {
		if m.stackCursor > 0 {
			m.stackCursor--
			m.clampStackScroll()
		}
	} else {
		if m.cursor > 0 {
			m.cursor--
			m.clampScroll()
		}
	}
}

func (m *goroutineModel) moveDown() {
	if m.drilled {
		if m.stackCursor < len(m.stackGroups)-1 {
			m.stackCursor++
			m.clampStackScroll()
		}
	} else {
		if m.cursor < len(m.groups)-1 {
			m.cursor++
			m.clampScroll()
		}
	}
}

func (m *goroutineModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *goroutineModel) visibleHeight() int {
	h := m.height - 4
	if h < 1 {
		h = 20
	}
	return h
}

func (m *goroutineModel) clampScroll() {
	vis := m.visibleHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *goroutineModel) clampStackScroll() {
	vis := m.visibleHeight()
	if m.stackCursor < m.stackOffset {
		m.stackOffset = m.stackCursor
	}
	if m.stackCursor >= m.stackOffset+vis {
		m.stackOffset = m.stackCursor - vis + 1
	}
}

func (m *goroutineModel) stateLabel() string {
	if m.stateIdx < 0 {
		return "all"
	}
	return m.allStates[m.stateIdx]
}

var (
	goroutineHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	goroutineCountStyle  = lipgloss.NewStyle().Bold(true)
	goroutineStackStyle  = lipgloss.NewStyle().Foreground(subtleColor)
)

func (m *goroutineModel) view() string {
	if len(m.goroutines) == 0 {
		return placeholderStyle.Render("\n  No goroutine data\n")
	}

	if m.drilled {
		return m.viewDrillDown()
	}
	return m.viewGroups()
}

func (m *goroutineModel) viewGroups() string {
	total := len(m.goroutines)
	var filtered int
	for _, g := range m.groups {
		filtered += g.Count
	}

	var b strings.Builder

	filterLine := fmt.Sprintf(" %d goroutines | filter: %s (%d shown) | [g] state  [enter] drill in  [q] quit",
		total, m.stateLabel(), filtered)
	b.WriteString(goroutineHeaderStyle.Render(filterLine))
	b.WriteByte('\n')
	b.WriteByte('\n')

	if len(m.groups) == 0 {
		b.WriteString(placeholderStyle.Render("  No goroutines match filter\n"))
		return b.String()
	}

	// Find the longest header for consistent cursor width.
	maxHeaderLen := 0
	for _, g := range m.groups {
		header := fmt.Sprintf("  %-20s %d goroutine", g.State, g.Count)
		if g.Count != 1 {
			header += "s"
		}
		if n := len([]rune(header)); n > maxHeaderLen {
			maxHeaderLen = n
		}
	}

	vis := m.visibleHeight()
	linesUsed := 0

	for i, g := range m.groups {
		if i < m.offset {
			continue
		}
		if linesUsed >= vis {
			break
		}

		isCursor := i == m.cursor

		header := fmt.Sprintf("  %-20s %d goroutine", g.State, g.Count)
		if g.Count != 1 {
			header += "s"
		}

		if isCursor {
			header = padRight(header, maxHeaderLen)
			b.WriteString(lipgloss.NewStyle().
				Background(selectBg).
				Foreground(selectFg).
				Bold(true).
				Render(header))
		} else {
			b.WriteString(goroutineCountStyle.Render(header))
		}
		b.WriteByte('\n')
		linesUsed++

		// Show goroutine ID and sample stack for the selected group.
		if isCursor && len(g.Goroutines) > 0 {
			idLine := fmt.Sprintf("    goroutine %d", g.Goroutines[0].ID)
			if len(g.Goroutines) > 1 {
				idLine += fmt.Sprintf(" (sample of %d)", len(g.Goroutines))
			}
			b.WriteString(goroutineStackStyle.Render(idLine))
			b.WriteByte('\n')
			linesUsed++
			linesUsed += m.renderStack(&b, g.Goroutines[0].Stack, vis-linesUsed)
		}
	}

	return b.String()
}

func (m *goroutineModel) viewDrillDown() string {
	var b strings.Builder

	var totalInGroup int
	for _, sg := range m.stackGroups {
		totalInGroup += sg.Count
	}

	headerLine := fmt.Sprintf(" %s — %d goroutines, %d unique stacks | [backspace] back",
		m.drillState, totalInGroup, len(m.stackGroups))
	b.WriteString(goroutineHeaderStyle.Render(headerLine))
	b.WriteByte('\n')
	b.WriteByte('\n')

	// Find the longest header for consistent cursor width.
	maxHeaderLen := 0
	for _, sg := range m.stackGroups {
		topFunc := ""
		if len(sg.Stack) > 0 {
			topFunc = sg.Stack[0].Func
		}
		header := fmt.Sprintf("  %d× %s", sg.Count, topFunc)
		if n := len([]rune(header)); n > maxHeaderLen {
			maxHeaderLen = n
		}
	}

	vis := m.visibleHeight()
	linesUsed := 0

	for i, sg := range m.stackGroups {
		if i < m.stackOffset {
			continue
		}
		if linesUsed >= vis {
			break
		}

		isCursor := i == m.stackCursor

		// Header: count + top function.
		topFunc := ""
		if len(sg.Stack) > 0 {
			topFunc = sg.Stack[0].Func
		}
		header := fmt.Sprintf("  %d× %s", sg.Count, topFunc)

		if isCursor {
			header = padRight(header, maxHeaderLen)
			b.WriteString(lipgloss.NewStyle().
				Background(selectBg).
				Foreground(selectFg).
				Bold(true).
				Render(header))
		} else {
			b.WriteString(goroutineCountStyle.Render(truncate(header, m.width)))
		}
		b.WriteByte('\n')
		linesUsed++

		// Show goroutine IDs and full stack for the selected stack group.
		if isCursor {
			b.WriteString(goroutineStackStyle.Render(truncate("    goroutines: "+formatIDs(sg.IDs), m.width)))
			b.WriteByte('\n')
			linesUsed++
			linesUsed += m.renderStack(&b, sg.Stack, vis-linesUsed)
		}
	}

	return b.String()
}

func (m *goroutineModel) renderStack(b *strings.Builder, stack []profile.StackFrame, maxLines int) int {
	lines := 0
	for _, frame := range stack {
		if lines >= maxLines {
			break
		}
		line := fmt.Sprintf("      %s", frame.Func)
		if frame.File != "" {
			line += fmt.Sprintf("  %s:%d", frame.File, frame.Line)
		}
		b.WriteString(goroutineStackStyle.Render(truncate(line, m.width)))
		b.WriteByte('\n')
		lines++
	}
	return lines
}

// formatIDs renders a list of goroutine IDs as a comma-separated string.
func formatIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ", ")
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}
