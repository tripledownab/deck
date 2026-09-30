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

// The rows the dashboard spends on chrome: the header over the columns, the
// footer under them, and the top border each column draws for itself. The rule
// that used to sit under the header is gone — the focused column's border is
// drawn in the accent colour instead.
const (
	dashboardHeaderRows = 1
	dashboardFooterRows = 1
	columnBorderRows    = 1
)

// dashboardLayout is where the dashboard puts its two columns. dashboardView
// sizes the columns from it and the mouse reads clicks against it.
//
// top is the one number the renderer does not read: the header and the
// borders land where they land. It is derived from the same row counts, and
// the click tests aim at the drawn frame, so a header that grew without it
// would fail them.
type dashboardLayout struct {
	navW    int // the project list's width
	detailW int // the detail column's width, the rest of the terminal
	top     int // screen row of each column's first line of content
	rows    int // lines of content in each column
}

func (m Model) dashboardLayout() dashboardLayout {
	navW := clamp(m.width/4, 22, 32)
	return dashboardLayout{
		navW:    navW,
		detailW: m.width - navW,
		top:     dashboardHeaderRows + columnBorderRows,
		rows:    m.height - dashboardHeaderRows - dashboardFooterRows - columnBorderRows,
	}
}

func (m Model) dashboardView() string {
	l := m.dashboardLayout()
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProjectList(l.navW, l.rows),
		m.renderProjectDetail(l.detailW, l.rows),
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
		return s.Footer.Render(" ↑/↓ session · ←/→ section · tab projects · ↵ open · n new · e rename · x close · ? help · q quit")
	}
	return s.Footer.Render(" ↑/↓ project · tab sessions · ↵ open · n new · a add project · e rename · ? help · q quit")
}
