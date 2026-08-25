package ui

// The session view's fixed rows: the top bar, the context bar naming the
// current session, and the footer that becomes a command menu while the
// prefix is armed.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/agent"
)

func (m Model) topBar() string {
	s := m.styles
	left := s.Wordmark.Render("◆ deck")

	tabs := []string{"Projects", "Sessions"}
	active := 1
	if m.screen == screenDashboard {
		active = 0
	}
	var rendered []string
	for i, t := range tabs {
		if i == active {
			rendered = append(rendered, s.TabActive.Render(t))
		} else {
			rendered = append(rendered, s.Tab.Render(t))
		}
	}

	working := 0
	for _, r := range m.runners {
		if r.Status() == agent.Working {
			working++
		}
	}
	right := s.Muted.Render(fmt.Sprintf("%d running", len(m.runners)))
	if working > 0 {
		right = s.Accent.Render(fmt.Sprintf("● %d working", working)) + s.Muted.Render(
			fmt.Sprintf(" · %d running", len(m.runners)))
	}
	right += s.Faint.Render("   ^g ?")

	return m.spread(left+"  "+lipgloss.JoinHorizontal(lipgloss.Top, rendered...), right)
}

func (m Model) contextBar() string {
	s := m.styles
	sess := m.currentSession()
	if sess == nil {
		return s.Muted.Render(" no sessions — press n to open one")
	}
	project := m.state.Project(sess.ProjectID)
	name := "?"
	if project != nil {
		name = project.Name
	}

	left := " " + s.Muted.Render(name+" / ") + s.Title.Render(sess.Name)

	var right []string
	right = append(right, s.Muted.Render(sess.Agent))
	if r := m.runners[sess.ID]; r != nil {
		right = append(right, s.Muted.Render(shortDuration(r.Uptime())))
	}
	if m.attached {
		right = append(right, s.Accent.Render("ATTACHED"))
	} else {
		right = append(right, s.Faint.Render("detached"))
	}
	if m.armed {
		right = append(right, s.Accent.Render("^g …"))
	}

	return m.spread(left, strings.Join(right, s.Faint.Render(" · "))+" ")
}

// commandHint is the footer shown while the prefix is armed.
//
// A prefix key that gives no feedback is a guessing game — you press ^g and
// the screen looks identical, so the only way to learn the second key is to
// remember it. Showing the menu the moment ^g is pressed makes the prefix
// teach itself, and is why the help modal is a reference rather than the only
// way to find a binding.
func (m Model) commandHint() string {
	s := m.styles
	pairs := [][2]string{
		{"d", "dashboard"}, {"j/k", "switch"}, {"1…9", "jump"}, {"n", "new"},
		{"x", "stop"}, {"↵", "attach"}, {"esc", "detach"}, {"?", "help"},
		{"q", "quit"}, {"^g", "literal"},
	}
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, s.Key.Render(p[0])+" "+s.Muted.Render(p[1]))
	}
	line := " " + s.Accent.Bold(true).Render("^g") + " " +
		strings.Join(parts, s.Faint.Render(" · "))
	return truncateStyled(line, m.width)
}

func (m Model) sessionFooter() string {
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
	if m.attached {
		return s.Footer.Render(" ^g for commands · ^g d dashboard · ^g j/k switch session · ^g n new · ^g ? help")
	}
	return s.Footer.Render(" ↑/↓ session · ↵ attach · n new · x stop · d dashboard · ? help · q quit")
}

// spread puts left and right on one line separated by padding, so the right
// group ends at the last column.
func (m Model) spread(left, right string) string {
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncateStyled(left, m.width)
	}
	return left + strings.Repeat(" ", gap) + right
}
