// Package tui contains the Bubble Tea program.
package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/owenrumney/lazypprof/internal/profile"
	"github.com/owenrumney/lazypprof/internal/source"
)

type view int

const (
	viewTop view = iota
	viewTree
	viewFlame
	viewGoroutine
)

func (v view) String() string {
	switch v {
	case viewTop:
		return "Top"
	case viewTree:
		return "Tree"
	case viewFlame:
		return "Flame"
	case viewGoroutine:
		return "Goroutines"
	}
	return "?"
}

// profileRefreshMsg is sent when a new profile arrives from live mode.
type profileRefreshMsg struct {
	prof *profile.Profile
	gen  uint64 // generation at the time the wait was registered
}

// modeSwitchMsg is sent when a mode switch completes (new profile type fetched).
type modeSwitchMsg struct {
	prof     *profile.Profile
	refreshC <-chan *profile.Profile
	cancel   context.CancelFunc
	pt       source.ProfileType
}

// modeSwitchErrMsg is sent when a mode switch fails.
type modeSwitchErrMsg struct {
	err error
}

// Model is the root Bubble Tea model.
type Model struct {
	prof      *profile.Profile
	view      view
	views     []view // available views for this profile type
	width     int
	height    int
	topTbl    table.Model
	tree      treeModel
	flame     flameModel
	goroutine goroutineModel
	refreshC  <-chan *profile.Profile // nil for file mode

	// Filter bar.
	filtering  bool
	filterInp  textinput.Model
	filterRe   *regexp.Regexp // nil = no active filter
	filterText string         // display string for active filter

	// Help overlay.
	showHelp bool

	// Live mode switching.
	liveConfig   *LiveConfig
	pollerCancel context.CancelFunc // cancel current poller
	refreshGen   uint64             // incremented on mode switch; drops stale refresh messages
	switching    string             // non-empty = "Switching to <type>..." overlay
	modePicker   bool               // true = mode picker overlay visible
	modePickIdx  int                // cursor in AllProfileTypes

	// Pause.
	paused      bool
	pendingProf *profile.Profile // latest profile received while paused

	// Diff mode.
	diffMode bool
	diffTop  diffTopView
}

// LiveConfig holds the parameters needed to switch profile types at runtime.
type LiveConfig struct {
	BaseURL     string
	Interval    time.Duration // 0 = auto-select based on ProfileType via source.DefaultInterval
	ProfileType source.ProfileType
}

// AllProfileTypes returns the available types in cycle order.
var AllProfileTypes = []source.ProfileType{
	source.ProfileCPU,
	source.ProfileHeap,
	source.ProfileAllocs,
	source.ProfileGoroutine,
	source.ProfileMutex,
	source.ProfileBlock,
}

// New constructs a Model for the given profile. Pass a non-nil channel for
// live mode to receive profile updates. Pass a non-nil LiveConfig to enable
// mode switching with [m].
func New(p *profile.Profile, refreshC <-chan *profile.Profile, opts ...Option) Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "regex filter..."
	ti.CharLimit = 100

	m := Model{
		prof:      p,
		topTbl:    newTopTable(p),
		tree:      newTreeModel(p),
		flame:     newFlameModel(p),
		goroutine: newGoroutineModel(p),
		refreshC:  refreshC,
		filterInp: ti,
	}
	for _, opt := range opts {
		opt(&m)
	}
	m.views = m.availableViews()
	m.view = m.views[0]
	return m
}

// Option configures the Model.
type Option func(*Model)

// WithLiveConfig enables mode switching in live mode.
func WithLiveConfig(cfg *LiveConfig, cancel context.CancelFunc) Option {
	return func(m *Model) {
		m.liveConfig = cfg
		m.pollerCancel = cancel
	}
}

// WithDiffMode marks this model as showing a diff between two profiles.
func WithDiffMode() Option {
	return func(m *Model) {
		m.diffMode = true
		m.diffTop = newDiffTopView(m.prof, nil)
	}
}

func (m *Model) availableViews() []view {
	if len(m.prof.Goroutines) > 0 {
		return []view{viewGoroutine, viewTop, viewTree, viewFlame}
	}
	return []view{viewTop, viewTree, viewFlame}
}

func (m Model) Init() tea.Cmd {
	if m.refreshC != nil {
		return m.waitForRefresh()
	}
	return nil
}

func (m Model) waitForRefresh() tea.Cmd {
	gen := m.refreshGen
	c := m.refreshC
	return func() tea.Msg {
		p, ok := <-c
		if !ok {
			return nil
		}
		return profileRefreshMsg{prof: p, gen: gen}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case profileRefreshMsg:
		if msg.gen != m.refreshGen {
			return m, nil // stale message from a previous mode's poller; discard
		}
		if m.paused {
			m.pendingProf = msg.prof // stash; apply when resumed
			return m, m.waitForRefresh()
		}
		m.refreshProfile(msg.prof)
		return m, m.waitForRefresh()

	case modeSwitchErrMsg:
		m.switching = ""
		return m, nil

	case modeSwitchMsg:
		m.switching = ""
		m.refreshGen++ // invalidate any in-flight profileRefreshMsg from old poller
		// Cancel old poller, wire up new one.
		if m.pollerCancel != nil {
			m.pollerCancel()
		}
		m.pollerCancel = msg.cancel
		m.refreshC = msg.refreshC
		m.liveConfig.ProfileType = msg.pt
		m.prof = msg.prof
		m.rebuildViews()
		return m, m.waitForRefresh()

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.topTbl.SetWidth(msg.Width - 2)
		m.topTbl.SetHeight(msg.Height - 4)
		m.diffTop.setSize(msg.Width-2, msg.Height-4)
		m.tree.setSize(msg.Width-2, msg.Height-4)
		m.flame.setSize(msg.Width-2, msg.Height-4)
		m.goroutine.setSize(msg.Width-2, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		// Mode picker captures keys when open.
		if m.modePicker {
			return m.updateModePicker(msg)
		}

		// Filter input mode captures all keys.
		if m.filtering {
			return m.updateFilter(msg)
		}

		// Help overlay: any key dismisses.
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Global keys.
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.cycleView()
			return m, nil
		case "s":
			m.cycleSampleType()
			return m, nil
		case "/":
			m.filtering = true
			m.filterInp.SetValue("")
			m.filterInp.Focus()
			return m, textinput.Blink
		case "?":
			m.showHelp = true
			return m, nil
		case "p":
			if m.refreshC != nil {
				m.paused = !m.paused
				if !m.paused && m.pendingProf != nil {
					m.refreshProfile(m.pendingProf)
					m.pendingProf = nil
				}
				return m, nil
			}
		case "m":
			if m.liveConfig != nil && m.switching == "" {
				m.modePicker = true
				// Pre-select current type.
				for i, pt := range AllProfileTypes {
					if pt == m.liveConfig.ProfileType {
						m.modePickIdx = i
						break
					}
				}
				return m, nil
			}
		case "esc", "escape":
			if m.filterRe != nil {
				m.clearFilter()
				return m, nil
			}
		}

		// Delegate view-local keys.
		if m.view == viewTop && m.diffMode {
			m.handleDiffTopKey(msg)
			return m, nil
		}
		if m.view == viewTree {
			m.handleTreeKey(msg)
			return m, nil
		}
		if m.view == viewFlame {
			m.handleFlameKey(msg)
			return m, nil
		}
		if m.view == viewGoroutine {
			m.handleGoroutineKey(msg)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.topTbl, cmd = m.topTbl.Update(msg)
	return m, cmd
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
		m.filterInp.Blur()
		text := m.filterInp.Value()
		if text == "" {
			m.clearFilter()
		} else {
			re, err := regexp.Compile("(?i)" + text)
			if err != nil {
				m.clearFilter()
			} else {
				m.filterRe = re
				m.filterText = text
				m.applyFilter()
			}
		}
		return m, nil
	case "esc", "escape":
		m.filtering = false
		m.filterInp.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.filterInp, cmd = m.filterInp.Update(msg)
	return m, cmd
}

func (m *Model) applyFilter() {
	m.topTbl = newTopTableFiltered(m.prof, m.filterRe)
	m.topTbl.SetWidth(m.width - 2)
	m.topTbl.SetHeight(m.height - 4)
	if m.diffMode {
		m.diffTop = newDiffTopView(m.prof, m.filterRe)
		m.diffTop.setSize(m.width-2, m.height-4)
	}
	m.tree.filterRe = m.filterRe
	m.tree.computeMatches()
	m.tree.expandToMatches()
	m.flame.filterRe = m.filterRe
	m.goroutine.filterRe = m.filterRe
}

func (m *Model) clearFilter() {
	m.filterRe = nil
	m.filterText = ""
	m.topTbl = newTopTable(m.prof)
	m.topTbl.SetWidth(m.width - 2)
	m.topTbl.SetHeight(m.height - 4)
	if m.diffMode {
		m.diffTop = newDiffTopView(m.prof, nil)
		m.diffTop.setSize(m.width-2, m.height-4)
	}
	m.tree.filterRe = nil
	m.tree.matched = nil
	m.tree.onPath = nil
	m.flame.filterRe = nil
	m.goroutine.filterRe = nil
}

func (m Model) View() string {
	var extras []string
	if m.diffMode {
		extras = append(extras, "diff")
	} else if m.liveConfig != nil {
		extras = append(extras, "live:"+string(m.liveConfig.ProfileType))
	} else if m.refreshC != nil {
		extras = append(extras, "live")
	}
	if m.filterText != "" {
		extras = append(extras, "filter: "+m.filterText)
	}
	extraStr := ""
	if len(extras) > 0 {
		extraStr = " │ " + strings.Join(extras, " │ ")
	}
	keys := "[tab] [/] [?] [q]"
	if m.refreshC != nil {
		keys = "[tab] [p] [/] [?] [q]"
	}
	if m.liveConfig != nil {
		keys = "[tab] [m] [p] [/] [?] [q]"
	}
	header := headerStyle.Render(fmt.Sprintf(
		" lazypprof │ %s │ %s (%s)%s │ %s ",
		m.view, m.prof.SampleType, m.prof.Unit(), extraStr, keys,
	))

	if m.showHelp {
		return lipgloss.JoinVertical(lipgloss.Left, header, m.helpView())
	}

	if m.modePicker {
		return lipgloss.JoinVertical(lipgloss.Left, header, m.modePickerView())
	}

	if m.switching != "" {
		overlay := lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Padding(2, 4).
			Render(m.switching)
		return lipgloss.JoinVertical(lipgloss.Left, header, overlay)
	}

	if m.prof.Empty() {
		if hint := m.emptyProfileHint(); hint != "" {
			body := placeholderStyle.Padding(2, 4).Render(hint)
			return lipgloss.JoinVertical(lipgloss.Left, header, body)
		}
	}

	var body string
	switch m.view {
	case viewTop:
		if m.diffMode {
			body = m.diffTop.view()
		} else {
			body = m.topTbl.View()
		}
	case viewTree:
		body = m.tree.view()
	case viewFlame:
		body = m.flame.view()
	case viewGoroutine:
		body = m.goroutine.view()
	}

	if m.filtering {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.filterInp.View())
	}

	if m.paused {
		banner := pauseBannerStyle.Width(m.width).Render("⏸  PAUSED — updates suspended  [p] to resume")
		body = lipgloss.JoinVertical(lipgloss.Left, banner, body)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *Model) handleDiffTopKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		m.diffTop.moveUp()
	case "down", "j":
		m.diffTop.moveDown()
	}
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

func (m *Model) cycleView() {
	for i, v := range m.views {
		if v == m.view {
			m.view = m.views[(i+1)%len(m.views)]
			return
		}
	}
	m.view = m.views[0]
}

func (m *Model) handleGoroutineKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		m.goroutine.moveUp()
	case "down", "j":
		m.goroutine.moveDown()
	case "g":
		m.goroutine.cycleState()
	case "enter":
		m.goroutine.enter()
	case "backspace":
		m.goroutine.back()
	}
}

func (m *Model) handleFlameKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "left", "h":
		m.flame.moveLeft()
	case "right", "l":
		m.flame.moveRight()
	case "up", "k":
		m.flame.moveDown()
	case "down", "j":
		m.flame.moveUp()
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
	if m.filterRe != nil {
		m.topTbl = newTopTableFiltered(m.prof, m.filterRe)
	} else {
		m.topTbl = newTopTable(m.prof)
	}
	m.topTbl.SetWidth(m.width - 2)
	m.topTbl.SetHeight(m.height - 4)
	m.tree = newTreeModel(m.prof)
	m.tree.filterRe = m.filterRe
	m.tree.computeMatches()
	m.tree.expandToMatches()
	m.tree.setSize(m.width-2, m.height-4)
	newFlame := newFlameModel(m.prof)
	newFlame.zoomRoot = m.flame.zoomRoot
	newFlame.zoomHist = m.flame.zoomHist
	newFlame.filterRe = m.filterRe
	m.flame = newFlame
	m.flame.setSize(m.width-2, m.height-4)
	oldGoroutine := m.goroutine
	m.goroutine = newGoroutineModel(m.prof)
	m.goroutine.filterRe = m.filterRe
	m.goroutine.stateIdx = oldGoroutine.stateIdx
	m.goroutine.rebuildGroups()
	m.goroutine.setSize(m.width-2, m.height-4)
	// Preserve drill-down: re-enter the same state if we were drilled in.
	if oldGoroutine.drilled {
		// Find the group matching the old drill state.
		for i, g := range m.goroutine.groups {
			if g.State == oldGoroutine.drillState {
				m.goroutine.cursor = i
				m.goroutine.enter()
				break
			}
		}
	}
	m.views = m.availableViews()
	// Ensure current view is still valid after the views list changed.
	found := false
	for _, v := range m.views {
		if v == m.view {
			found = true
			break
		}
	}
	if !found {
		m.view = m.views[0]
	}
}

func (m *Model) updateModePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.modePickIdx > 0 {
			m.modePickIdx--
		}
		return m, nil
	case "down", "j":
		if m.modePickIdx < len(AllProfileTypes)-1 {
			m.modePickIdx++
		}
		return m, nil
	case "enter":
		selected := AllProfileTypes[m.modePickIdx]
		m.modePicker = false
		if selected == m.liveConfig.ProfileType {
			return m, nil // no change
		}
		m.switching = fmt.Sprintf("Switching to %s...", selected)
		return m, m.switchModeTo(selected)
	case "esc", "escape", "q", "m":
		m.modePicker = false
		return m, nil
	}
	return m, nil
}

func (m *Model) switchModeTo(pt source.ProfileType) tea.Cmd {
	cfg := m.liveConfig

	return func() tea.Msg {
		httpSrc := source.NewHTTPSource(cfg.BaseURL, pt)
		prof, err := httpSrc.Load()
		if err != nil {
			return modeSwitchErrMsg{err: err}
		}

		interval := cfg.Interval
		if interval == 0 {
			interval = source.DefaultInterval(pt)
		}
		poller := source.NewPoller(httpSrc, interval)
		ctx, cancel := context.WithCancel(context.Background())
		go poller.Run(ctx)

		return modeSwitchMsg{
			prof:     prof,
			refreshC: poller.C,
			cancel:   cancel,
			pt:       pt,
		}
	}
}

func (m Model) modePickerView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("  Select profile type"))
	b.WriteString("\n\n")

	for i, pt := range AllProfileTypes {
		cursor := "  "
		if i == m.modePickIdx {
			cursor = "▸ "
		}

		label := fmt.Sprintf("%s%s", cursor, pt)
		if pt == m.liveConfig.ProfileType {
			label += " (current)"
		}

		if i == m.modePickIdx {
			label = padRight(label, 40)
			b.WriteString(lipgloss.NewStyle().
				Background(selectBg).
				Foreground(selectFg).
				Bold(true).
				Render(label))
		} else {
			b.WriteString("  " + string(pt))
			if pt == m.liveConfig.ProfileType {
				b.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(" (current)"))
			}
		}
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("  enter: select  esc: cancel"))
	return b.String()
}

func (m Model) emptyProfileHint() string {
	if m.liveConfig == nil {
		return "No sample data in this profile."
	}
	switch m.liveConfig.ProfileType {
	case source.ProfileMutex:
		return "No mutex contention data.\nIs runtime.SetMutexProfileFraction() enabled in the target service?"
	case source.ProfileBlock:
		return "No block profile data.\nIs runtime.SetBlockProfileRate() enabled in the target service?"
	default:
		return "No sample data in this profile."
	}
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
