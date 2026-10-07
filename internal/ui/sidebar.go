package ui

// The session sidebar: the scrolling column of project headings and session
// cards down the left of the session view. card.go draws each card.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/store"
)

// sidebarRow is one line group in the session sidebar: a project header when
// session is nil, otherwise a session card.
type sidebarRow struct {
	project *store.Project
	session *store.Session
}

func (m Model) renderSidebar(width, height int) string {
	lines, shown, _ := m.sidebarWindow(width, height)
	out := fill(shown, texts(lines), m.styles.Faint.Render(moreGlyph))
	for i, l := range out {
		out[i] = " " + l
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(out, "\n"))
}

// sidebarWindow is the sidebar laid into height rows: its lines, which line
// each row shows, and the first line shown. The renderer and the mouse both
// read it, so a click lands on the card drawn under the pointer.
//
// The window keeps the selected card's last line in view and then its first,
// so a card that fits is shown whole. Keeping only the first would leave the
// rest of a card at the bottom edge behind the marker.
func (m Model) sidebarWindow(width, height int) (lines []drawnLine, shown []int, start int) {
	lines, first, last := m.sidebarLines(width)
	_, start = windowed(len(lines), last, height, true, m.sidebarTop)
	shown, start = windowed(len(lines), first, height, true, start)
	return lines, shown, start
}

// sidebarLines is the sidebar before it is windowed: a heading per project and
// a card per session, each card line standing for its row in m.rows. first and
// last are the selected card's first and last lines.
func (m Model) sidebarLines(width int) (lines []drawnLine, first, last int) {
	s := m.styles
	inner := width - 1 // one column of gutter before the pane border

	nth := 0 // sessions only, so it matches the ^g 1…9 the user presses
	for i, row := range m.rows {
		if row.session == nil {
			if len(lines) > 0 {
				lines = append(lines, drawnLine{})
			}
			lines = append(lines, drawnLine{text: s.GroupLabel.Render(
				truncate(strings.ToUpper(row.project.Name), inner-2))})
			continue
		}
		nth++
		if i == m.rowIx {
			first = len(lines)
		}
		// The jump numbers show only while the prefix is armed, for the same
		// reason commandHint does: a binding you cannot see the targets of is
		// a guessing game, and a digit on every card the rest of the time
		// spends two columns of title on something you are not doing.
		label := 0
		if m.armed {
			label = nth
		}
		for _, l := range m.sessionCard(row.session, i == m.rowIx, inner, label) {
			lines = append(lines, wholeLine(l, target{hitRow, i}))
		}
		if i == m.rowIx {
			last = len(lines) - 1
		}
	}

	if len(lines) == 0 {
		lines = []drawnLine{
			{text: s.Muted.Render("No sessions yet.")},
			{},
			{text: s.Faint.Render("Press n to open one.")},
		}
	}
	return lines, first, last
}
