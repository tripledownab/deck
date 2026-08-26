// Package ui is Deck's Bubble Tea program: a dashboard over projects and
// sessions, and a session view whose main pane is a live agent terminal.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// frameInterval is how often the agent pane is repainted. Bubble Tea redraws
// on every message anyway, so this only bounds how stale PTY output can look;
// 50ms is below the threshold where typing feels laggy.
const frameInterval = 50 * time.Millisecond

type frameMsg time.Time

func frameTick() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg { return frameMsg(t) })
}

type screen int

const (
	screenDashboard screen = iota
	screenSession
)

// column is which half of the dashboard has keyboard focus.
type column int

const (
	colProjects column = iota
	colContent
)

func (m Model) Init() tea.Cmd { return frameTick() }

// Close stops every agent process.
//
// Call this after tea.Program.Run has returned, never from Update: Stop waits
// on the child, and waiting inside the update loop stalls the loop that would
// otherwise keep draining output. This is the same constraint cathode
// documents for Engine.Close.
func (m Model) Close() {
	for _, r := range m.runners {
		r.Stop()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizePane()
		return m, nil

	case frameMsg:
		m.resizePane()
		m.sweepExited()
		return m, frameTick()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "" // pre-layout; Bubble Tea sends WindowSizeMsg immediately
	}
	if m.picker != nil {
		return m.picker.view(m.styles, m.width, m.height)
	}
	if m.browser != nil {
		return m.browser.view(m.styles, m.width, m.height)
	}
	if m.form != nil {
		return m.form.view(m.styles, m.width, m.height)
	}
	if m.showHelp {
		return m.helpView()
	}
	if m.screen == screenSession {
		return m.sessionView()
	}
	return m.dashboardView()
}
