// Package tui contains the Bubble Tea program.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
)

type view int

const (
	viewTop view = iota
	viewTree
	viewFlame
	numViews
)

func (v view) String() string {
	switch v {
	case viewTop:
		return "Top"
	case viewTree:
		return "Tree"
	case viewFlame:
		return "Flame"
	}
	return "?"
}

// profileRefreshMsg is sent when a new profile arrives from live mode.
type profileRefreshMsg struct {
	prof *profile.Profile
}

// Model is the root Bubble Tea model.
type Model struct {
	prof    *profile.Profile
	view    view
	width   int
	height  int
	topTbl  table.Model
	tree    treeModel
	flame   flameModel
	refreshC <-chan *profile.Profile // nil for file mode
}

// New constructs a Model for the given profile. Pass a non-nil channel for
// live mode to receive profile updates.
func New(p *profile.Profile, refreshC <-chan *profile.Profile) Model {
	return Model{
		prof:     p,
		view:     viewTop,
		topTbl:   newTopTable(p),
		tree:     newTreeModel(p),
		flame:    newFlameModel(p),
		refreshC: refreshC,
	}
}

func (m Model) Init() tea.Cmd {
	if m.refreshC != nil {
		return m.waitForRefresh()
	}
	return nil
}

func (m Model) waitForRefresh() tea.Cmd {
	return func() tea.Msg {
		p, ok := <-m.refreshC
		if !ok {
			return nil
		}
		return profileRefreshMsg{prof: p}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case profileRefreshMsg:
		m.refreshProfile(msg.prof)
		return m, m.waitForRefresh()

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.topTbl.SetWidth(msg.Width - 2)
		m.topTbl.SetHeight(msg.Height - 4)
		m.tree.setSize(msg.Width-2, msg.Height-4)
		m.flame.setSize(msg.Width-2, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.view = (m.view + 1) % numViews
			return m, nil
		case "s":
			m.cycleSampleType()
			return m, nil
		}

		// Delegate view-local keys.
		if m.view == viewTree {
			m.handleTreeKey(msg)
			return m, nil
		}
		if m.view == viewFlame {
			m.handleFlameKey(msg)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.topTbl, cmd = m.topTbl.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	mode := ""
	if m.refreshC != nil {
		mode = " │ live"
	}
	header := headerStyle.Render(fmt.Sprintf(
		" lazypprof │ view: %s │ sample: %s (%s)%s │ [tab] view  [s] sample  [q] quit ",
		m.view, m.prof.SampleType, m.prof.Unit(), mode,
	))

	var body string
	switch m.view {
	case viewTop:
		body = m.topTbl.View()
	case viewTree:
		body = m.tree.view()
	case viewFlame:
		body = m.flame.view()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *Model) handleTreeKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		m.tree.moveUp()
	case "down", "j":
		m.tree.moveDown()
	case "right", "l", "enter":
		m.tree.expand()
	case "left", "h":
		m.tree.collapse()
	case " ":
		m.tree.toggle()
	case "0":
		m.tree.collapseAll()
	case "*":
		m.tree.expandSubtree()
	}
}

func (m *Model) handleFlameKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "left", "h":
		m.flame.moveLeft()
	case "right", "l":
		m.flame.moveRight()
	case "up", "k":
		m.flame.moveUp()
	case "down", "j":
		m.flame.moveDown()
	case "enter":
		m.flame.zoomIn()
	case "backspace":
		m.flame.zoomOut()
	case "0":
		m.flame.zoomReset()
	}
}

func (m *Model) refreshProfile(p *profile.Profile) {
	// Preserve the current sample type if it exists in the new profile.
	oldType := m.prof.SampleType
	m.prof = p
	if !m.prof.SetSampleType(oldType) {
		// Fall back to default (last sample type).
		types := m.prof.SampleTypes()
		if len(types) > 0 {
			m.prof.SetSampleType(types[len(types)-1])
		}
	}
	m.rebuildViews()
}

func (m *Model) rebuildViews() {
	m.topTbl = newTopTable(m.prof)
	m.topTbl.SetWidth(m.width - 2)
	m.topTbl.SetHeight(m.height - 4)
	m.tree = newTreeModel(m.prof)
	m.tree.setSize(m.width-2, m.height-4)
	newFlame := newFlameModel(m.prof)
	newFlame.zoomRoot = m.flame.zoomRoot
	newFlame.zoomHist = m.flame.zoomHist
	m.flame = newFlame
	m.flame.setSize(m.width-2, m.height-4)
}

func (m *Model) cycleSampleType() {
	types := m.prof.SampleTypes()
	if len(types) <= 1 {
		return
	}
	for i, t := range types {
		if t == m.prof.SampleType {
			m.prof.SetSampleType(types[(i+1)%len(types)])
			m.rebuildViews()
			return
		}
	}
}
