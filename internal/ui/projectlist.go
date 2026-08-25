package ui

// The dashboard's left column: the project list, its cursor, and the border
// that marks which column has focus.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/store"
)

// columnStyle frames a dashboard column, colouring its top border to show
// which one the arrow keys drive.
//
// A whole-column background was the alternative. It loses against a border on
// a pane that hosts arbitrary content: a background has to be re-asserted on
// every padded cell, and it fights whatever theme the terminal already has.
// One coloured rule costs a single row and reads from across the desk.
func (m Model) columnStyle(focused bool) lipgloss.Style {
	colour := m.styles.P.Border
	if focused {
		colour = m.styles.P.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.Border{Top: "─"}, true, false, false, false).
		BorderForeground(colour)
}

func (m Model) renderProjectList(width, height int) string {
	s := m.styles
	inner := width - 2

	header := s.GroupLabel.Render("PROJECTS")
	if m.focus == colProjects {
		header = s.Accent.Bold(true).Render("PROJECTS")
	}
	lines := []string{" " + header, ""}

	if len(m.state.Projects) == 0 {
		lines = append(lines,
			" "+s.Faint.Render("none yet"),
			"",
			" "+s.Faint.Render("press a to add"))
	}

	for i := range m.state.Projects {
		p := &m.state.Projects[i]
		marker, style := m.cursorMarker(i == m.projectIx, m.focus == colProjects)
		count := len(m.state.SessionsFor(p.ID))
		label := truncate(p.Name, inner-6)
		row := marker + style.Render(label)
		if count > 0 {
			row += s.Faint.Render(fmt.Sprintf(" %d", count))
		}
		lines = append(lines, " "+row)
	}

	bodyH := height - 1 // the top border takes a row
	lines = window(lines, m.projectIx+2, bodyH)
	return m.columnStyle(m.focus == colProjects).
		Width(width).Height(bodyH).
		Render(strings.Join(lines, "\n"))
}

// projectStatus reports the liveliest state among the project's sessions.
func (m Model) projectStatus(p *store.Project) string {
	running := 0
	for _, sess := range m.state.SessionsFor(p.ID) {
		if _, ok := m.runners[sess.ID]; ok {
			running++
		}
	}
	if running > 0 {
		return fmt.Sprintf("Active · %d running", running)
	}
	return "Idle"
}

// cursorMarker renders the row cursor for a list.
//
// The unfocused column keeps a dimmed cursor rather than losing it. Both
// columns hold a position at all times, and hiding the inactive one made tab
// look like it did nothing but recolour a heading.
func (m Model) cursorMarker(selected, focused bool) (marker string, style lipgloss.Style) {
	s := m.styles
	switch {
	case selected && focused:
		return s.Accent.Render("▸ "), s.Value.Bold(true)
	case selected:
		return s.Faint.Render("· "), s.Value
	default:
		return "  ", s.Muted
	}
}
