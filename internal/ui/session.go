package ui

// The session view: its layout arithmetic and the frame that composes the
// sidebar, the pane and the chrome around them.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Chrome rows the session view spends: top bar, context bar and footer, of
// which the first two sit above the body.
const (
	sessionChromeRows = 3
	sessionBodyTop    = 2
)

// layout returns the sidebar width and the body height for the session view.
// Everything that needs these numbers calls this, so the emulator size and the
// drawn frame cannot disagree.
func (m Model) layout() (sidebarW, bodyH int) {
	sidebarW = clamp(m.width/4, 24, 34)
	if m.width < 60 {
		sidebarW = 0 // too narrow to split; the pane takes the screen
	}
	bodyH = m.height - sessionChromeRows
	return sidebarW, bodyH
}

// paneChromeCols is what the pane frame costs horizontally: one column for the
// left rule and one for the padding beside it. It costs nothing vertically,
// which is the point of framing with a rule rather than a box.
const paneChromeCols = 2

// paneSize is the agent terminal's size in cells: the body minus the sidebar
// and the pane frame.
func (m Model) paneSize() (w, h int) {
	sidebarW, bodyH := m.layout()
	return max(m.width-sidebarW-paneChromeCols, 20), max(bodyH, 5)
}

// sessionView draws the session view at exactly the terminal's height.
//
// The pane is drawn at the body's height, not at paneSize's. paneSize keeps
// the agent's terminal at least five rows, and drawing all of them on a shorter
// terminal made the frame taller than the screen. Bubble Tea then drops the
// top rows, so the sidebar was drawn a row higher than a click resolved it.
func (m Model) sessionView() string {
	sidebarW, bodyH := m.layout()
	paneW, _ := m.paneSize()

	parts := []string{m.topBar(), m.contextBar()}
	if bodyH > 0 {
		body := m.renderPane(paneW, bodyH)
		if sidebarW > 0 {
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(sidebarW, bodyH), body)
		}
		parts = append(parts, body)
	}
	return strings.Join(append(parts, m.sessionFooter()), "\n")
}
