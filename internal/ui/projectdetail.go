package ui

// The dashboard's right column: the selected project's metadata, its
// Overview tab, and its session list.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/store"
)

// detailIndent is the blank margin at the start of every detail line, between
// the column's left edge and its text.
const detailIndent = 1

// renderProjectDetail draws the right column: rows lines of content under the
// column's top border.
func (m Model) renderProjectDetail(width, rows int) string {
	column := m.columnStyle(m.focus == colContent).Width(width).Height(rows)
	lines := m.detailLines(width)
	if lines == nil {
		return column.Render(m.placeholder(width, rows,
			"No projects registered.",
			"Press a to add a git repository."))
	}
	text := clip(texts(lines), rows)
	margin := strings.Repeat(" ", detailIndent)
	for i, l := range text {
		text[i] = margin + truncate(l, width-detailIndent)
	}
	return column.Render(strings.Join(text, "\n"))
}

// detailLines is the detail column from the top, before it is clipped to the
// column's height: the project's heading and metadata, the section tabs, and
// the selected section. The renderer draws from it and the mouse reads clicks
// against it. Nil when there is no project to show.
func (m Model) detailLines(width int) []drawnLine {
	s := m.styles
	inner := width - 3
	p := m.currentProject()
	if p == nil {
		return nil
	}

	lines := []drawnLine{{text: s.Muted.Render("Projects › ") + s.Title.Render(p.Name)}}
	if p.Description != "" {
		lines = append(lines, drawnLine{text: s.Subtitle.Render(truncate(p.Description, inner))})
	}

	sessions := m.state.SessionsFor(p.ID)
	lines = append(lines,
		drawnLine{text: m.metaRow(map[string]string{}, []metaItem{
			{"Status", m.projectStatus(p)},
			{"Sessions", fmt.Sprint(len(sessions))},
			{"Added", ago(p.CreatedAt)},
			{"Path", truncate(p.Path, max(inner-46, 12))},
		})},
		drawnLine{},
		m.tabLine(),
		drawnLine{text: s.Rule.Render(strings.Repeat("─", max(inner, 0)))},
		drawnLine{},
	)

	switch dashboardTabs[m.tabIx] {
	case "Overview":
		lines = append(lines, m.overviewBody(sessions, inner)...)
	case "Sessions":
		lines = append(lines, m.sessionsBody(sessions, inner)...)
	}
	return lines
}

// tabLine is the section tabs, each standing for the cells its label covers.
// The focus marker leads, so the right column advertises the keyboard the same
// way the PROJECTS heading does.
func (m Model) tabLine() drawnLine {
	s := m.styles
	marker := " "
	if m.focus == colContent {
		marker = s.Accent.Render("▸")
	}
	parts := []string{marker}
	x := lipgloss.Width(marker)
	var spans []span
	for i, t := range dashboardTabs {
		style := s.Tab
		if i == m.tabIx {
			style = s.TabActive
		}
		label := style.Render(t)
		w := lipgloss.Width(label)
		spans = append(spans, span{x, x + w, target{hitTab, i}})
		parts = append(parts, label)
		x += w
	}
	return drawnLine{text: lipgloss.JoinHorizontal(lipgloss.Top, parts...), spans: spans}
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

func (m Model) overviewBody(sessions []store.Session, width int) []drawnLine {
	s := m.styles
	lines := []drawnLine{
		{text: s.Title.Render("Sessions") + " " + s.Faint.Render(fmt.Sprint(len(sessions)))},
		{},
	}
	if len(sessions) == 0 {
		return append(lines, drawnLine{text: s.Faint.Render("None yet. Press n to open one.")})
	}
	limit := min(len(sessions), 6)
	for i := range sessions {
		if i == limit {
			return append(lines, drawnLine{},
				drawnLine{text: s.Faint.Render(fmt.Sprintf("+ %d more — → for the Sessions tab", len(sessions)-limit))})
		}
		lines = append(lines, wholeLine(
			m.sessionLine(&sessions[i], i == m.listIx, m.focus == colContent, width),
			target{hitSession, i}))
	}
	return lines
}

// sessionsBody gives each session two lines, its row and its branch, and both
// stand for the session: they are one entry to the eye.
func (m Model) sessionsBody(sessions []store.Session, width int) []drawnLine {
	s := m.styles
	if len(sessions) == 0 {
		return []drawnLine{{text: s.Faint.Render("No sessions. Press n to open one.")}}
	}
	var lines []drawnLine
	for i := range sessions {
		t := target{hitSession, i}
		lines = append(lines,
			wholeLine(m.sessionLine(&sessions[i], i == m.listIx, m.focus == colContent, width), t),
			wholeLine("   "+s.Faint.Render(truncate("⑂ "+refOf(&sessions[i]), width-4)), t))
	}
	return lines
}

// sessionLine is one row of a session list: marker, prompt glyph, title,
// status, age.
func (m Model) sessionLine(sess *store.Session, selected, focused bool, width int) string {
	s := m.styles

	marker, titleStyle := m.cursorMarker(selected, focused)

	// The detail joins the label here rather than taking a line of its own as
	// it does on a sidebar card. This row has width to spend and no second
	// line to spend instead, and an exited session that does not say why is a
	// dead end.
	st := m.statusOf(sess)
	text := st.label
	if st.detail != "" {
		text += ": " + st.detail
	}
	right := st.style.Render(st.glyph+" "+text) + s.Faint.Render("  "+ago(sess.CreatedAt))

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
