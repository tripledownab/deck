package ui

// The agent pane: the emulated terminal screen, and what stands in for it
// when a session is closed or its process has gone.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/agent"
)

func (m Model) renderPane(width, height int) string {
	s := m.styles
	border := s.Pane
	if m.attached {
		border = s.PaneActive
	}

	sess := m.currentSession()
	if sess == nil {
		return border.Render(m.placeholder(width, height,
			"No session selected.",
			"Press n to open one, or ^g d for the dashboard."))
	}

	r := m.runners[sess.ID]
	if r == nil {
		return border.Render(m.placeholder(width, height,
			"“"+firstLine(sess.Title, sess.Name)+"” is closed.",
			"Press ↵ to start "+sess.Agent+" in "+sess.Dir))
	}
	if r.Status() == agent.Exited {
		reason := "the agent exited."
		if err := r.Err(); err != nil {
			reason = "the agent exited: " + err.Error()
		}
		// The emulator still holds the final screen, so show it under a
		// banner rather than clearing what the agent last said.
		lines := r.Render(false, nil, nil)
		lines = append([]string{s.Error.Render(truncate("! "+reason, width))}, lines...)
		return border.Render(strings.Join(clip(lines, height), "\n"))
	}

	lines := r.Render(m.attached, m.styles.P.CursorFg, m.styles.P.CursorBg)
	return border.Render(strings.Join(clip(lines, height), "\n"))
}

func (m Model) placeholder(width, height int, title, hint string) string {
	s := m.styles
	body := lipgloss.JoinVertical(lipgloss.Center,
		s.Muted.Render(title),
		"",
		s.Faint.Render(hint),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}
