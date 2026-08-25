package ui

// The dashboard's right column: the selected project's metadata, its
// Overview tab, and its session list.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/store"
)

func (m Model) renderProjectDetail(width, height int) string {
	s := m.styles
	inner := width - 3
	bodyH := height - 1 // the top border takes a row
	column := m.columnStyle(m.focus == colContent).Width(width).Height(bodyH)

	p := m.currentProject()
	if p == nil {
		return column.Render(m.placeholder(width, bodyH,
			"No projects registered.",
			"Press a to add a git repository."))
	}

	var b strings.Builder
	b.WriteString(s.Muted.Render("Projects › ") + s.Title.Render(p.Name) + "\n")
	if p.Description != "" {
		b.WriteString(s.Subtitle.Render(truncate(p.Description, inner)) + "\n")
	}

	sessions := m.state.SessionsFor(p.ID)
	b.WriteString(m.metaRow(map[string]string{}, []metaItem{
		{"Status", m.projectStatus(p)},
		{"Sessions", fmt.Sprint(len(sessions))},
		{"Added", ago(p.CreatedAt)},
		{"Path", truncate(p.Path, max(inner-46, 12))},
	}) + "\n\n")

	// Section tabs. The focus marker sits beside them so the right column
	// advertises the keyboard the same way the PROJECTS heading does.
	var tabs []string
	if m.focus == colContent {
		tabs = append(tabs, s.Accent.Render("▸"))
	} else {
		tabs = append(tabs, " ")
	}
	for i, t := range dashboardTabs {
		if i == m.tabIx {
			tabs = append(tabs, s.TabActive.Render(t))
		} else {
			tabs = append(tabs, s.Tab.Render(t))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...) + "\n")
	b.WriteString(s.Rule.Render(strings.Repeat("─", inner)) + "\n\n")

	switch dashboardTabs[m.tabIx] {
	case "Overview":
		b.WriteString(m.overviewBody(p, sessions, inner))
	case "Sessions":
		b.WriteString(m.sessionsBody(sessions, inner))
	}

	lines := clip(strings.Split(b.String(), "\n"), bodyH)
	for i, l := range lines {
		lines[i] = " " + l
	}
	return column.Render(strings.Join(lines, "\n"))
}

type metaItem struct{ label, value string }

func (m Model) metaRow(_ map[string]string, items []metaItem) string {
	s := m.styles
	var parts []string
	for _, it := range items {
		if it.value == "" {
			continue
		}
		parts = append(parts, s.Label.Render(it.label)+" "+s.Value.Render(it.value))
	}
	return strings.Join(parts, s.Faint.Render("   "))
}

func (m Model) overviewBody(p *store.Project, sessions []store.Session, width int) string {
	s := m.styles
	var b strings.Builder

	b.WriteString(s.Title.Render("Sessions") + " " + s.Faint.Render(fmt.Sprint(len(sessions))) + "\n\n")
	if len(sessions) == 0 {
		b.WriteString(s.Faint.Render("None yet. Press n to open one.") + "\n")
		return b.String()
	}
	limit := min(len(sessions), 6)
	for i := range sessions {
		if i == limit {
			b.WriteString("\n" + s.Faint.Render(fmt.Sprintf("+ %d more — → for the Sessions tab", len(sessions)-limit)) + "\n")
			break
		}
		b.WriteString(m.sessionLine(&sessions[i], i == m.listIx, m.focus == colContent, width) + "\n")
	}
	return b.String()
}

func (m Model) sessionsBody(sessions []store.Session, width int) string {
	s := m.styles
	if len(sessions) == 0 {
		return s.Faint.Render("No sessions. Press n to open one.")
	}
	var b strings.Builder
	for i := range sessions {
		b.WriteString(m.sessionLine(&sessions[i], i == m.listIx, m.focus == colContent, width) + "\n")
		b.WriteString("   " + s.Faint.Render(truncate("⑂ "+refOf(&sessions[i]), width-4)) + "\n")
	}
	return b.String()
}

// sessionLine is one row of a session list: marker, prompt glyph, title,
// status, age.
func (m Model) sessionLine(sess *store.Session, selected, focused bool, width int) string {
	s := m.styles

	marker, titleStyle := m.cursorMarker(selected, focused)

	glyph, label, style := m.statusOf(sess)
	right := style.Render(glyph+" "+label) + s.Faint.Render("  "+ago(sess.CreatedAt))

	title := firstLine(sess.Title, sess.Name)
	left := marker + s.Faint.Render(">_ ") + titleStyle.Render(title)

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		left = truncateStyled(left, max(width-lipgloss.Width(right)-2, 8))
		gap = max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	}
	return left + strings.Repeat(" ", gap) + right
}

func refOf(sess *store.Session) string {
	if sess.Branch != "" {
		return sess.Branch
	}
	return sess.Dir
}
