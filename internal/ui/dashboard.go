package ui

// The dashboard view: its tabs, chrome and the two-column frame that
// projectlist.go and projectdetail.go fill.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// dashboardTabs are the sections of a project.
//
// The reference portal also shows Wiki, Tasks, Catalog, Resources, PRs,
// Automations, People, and Activity. Those are views onto a service catalog
// and an issue tracker that a local tool has nothing to fill them with, so
// Deck ships the two it can answer honestly and leaves the rest out
// rather than drawing empty tabs.
var dashboardTabs = []string{"Overview", "Sessions"}

// The dashboard spends two rows on chrome: the header and the footer. The rule
// that used to sit under the header is gone — each column draws its own top
// border now, and the focused one draws it in the accent colour.
const dashboardChromeRows = 2

func (m Model) dashboardView() string {
	navW := clamp(m.width/4, 22, 32)
	bodyH := m.height - dashboardChromeRows

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProjectList(navW, bodyH),
		m.renderProjectDetail(m.width-navW, bodyH),
	)

	return strings.Join([]string{
		m.dashboardHeader(),
		body,
		m.dashboardFooter(),
	}, "\n")
}

func (m Model) dashboardHeader() string {
	s := m.styles
	left := " " + s.Wordmark.Render("◆ deck")
	right := s.Muted.Render(fmt.Sprintf("%s · %s · %d running ",
		plural(len(m.state.Projects), "project"),
		plural(len(m.state.Sessions), "session"),
		len(m.runners)))
	return m.spread(left, right)
}

func (m Model) dashboardFooter() string {
	s := m.styles
	if m.armed {
		return m.commandHint()
	}
	if m.fault != nil {
		return s.Error.Render(" ! " + truncate(m.fault.Error(), m.width-4))
	}
	if m.notice != "" {
		return s.Muted.Render(" " + truncate(m.notice, m.width-2))
	}
	// Name what the arrows move and where tab goes. "tab column" told the user
	// nothing about which column they were leaving.
	// Only advertise ←/→ as section keys where they are. From the projects
	// list they step into the detail column, and saying "section" there was
	// the footer describing the bug rather than the behaviour.
	if m.focus == colContent {
		return s.Footer.Render(" ↑/↓ session · ←/→ section · tab projects · ↵ open · n new session · x close · ? help · q quit")
	}
	return s.Footer.Render(" ↑/↓ project · tab sessions · ↵ open · n new session · a add project · ? help · q quit")
}
