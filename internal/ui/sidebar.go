package ui

// The session sidebar: the scrolling column of project headings and session
// cards down the left of the session view.

import (
	"fmt"
	"strconv"
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
	s := m.styles
	inner := width - 1 // one column of gutter before the pane border

	var lines []string
	selected := 0
	nth := 0 // sessions only, so it matches the ^g 1…9 the user presses
	for i, row := range m.rows {
		if row.session == nil {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, s.GroupLabel.Render(
				truncate(strings.ToUpper(row.project.Name), inner-2)))
			continue
		}
		nth++
		if i == m.rowIx {
			selected = len(lines)
		}
		// The jump numbers show only while the prefix is armed, for the same
		// reason commandHint does: a binding you cannot see the targets of is
		// a guessing game, and a digit on every card the rest of the time
		// spends two columns of title on something you are not doing.
		label := 0
		if m.armed {
			label = nth
		}
		lines = append(lines, m.sessionCard(row.session, i == m.rowIx, inner, label)...)
	}

	if len(lines) == 0 {
		lines = []string{
			s.Muted.Render("No sessions yet."),
			"",
			s.Faint.Render("Press n to open one."),
		}
	}

	lines = window(lines, selected, height)
	for i, l := range lines {
		lines[i] = " " + l
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

// sessionCard renders one session as three lines: title, branch, status.
//
// A nth of 1..9 prefixes the title with that jump number; anything else, zero
// included, leaves the card unnumbered and the title two columns longer.
func (m Model) sessionCard(sess *store.Session, active bool, width, nth int) []string {
	s := m.styles

	bar := "  "
	titleStyle := s.Value
	if active {
		bar = s.Accent.Render("▌ ")
		titleStyle = s.Value.Bold(true)
	}

	// The number sits on the title line only. Indenting all three lines under
	// it would redraw the whole card on every ^g, and the branch and status
	// lines are already keyed to the bar.
	num, numW := "", 0
	if nth >= 1 && nth <= 9 {
		num, numW = s.Accent.Render(strconv.Itoa(nth))+" ", 2
	}

	title := sess.Title
	if title == "" {
		title = sess.Name
	}

	ref := sess.Branch
	if ref == "" {
		ref = "· project directory"
	}

	glyph, label, style := m.statusOf(sess)

	// Claims and waiting mail are the two things about a sibling worth seeing
	// at a glance: one shows two agents in the same files, the other shows a
	// message nobody has collected. Mail is pull-only, so without this an
	// unread message is invisible until the agent happens to ask.
	// Mail first, because the status line truncates from the right and the two
	// badges are not equally urgent. Unread mail means a sibling asked for
	// something and nobody has read it; a claim is a fact about files.
	//
	// Both reach agents by other routes — coord.Siblings reports claims, and
	// mcp.callTool appends the unread count to every tool result except the
	// inbox's own. Neither reaches the person scanning the sidebar any other
	// way, so the order here decides only what a narrow column drops.
	if m.coord != nil {
		if n := m.coord.Unread(sess.ID); n > 0 {
			label += s.Accent.Render(fmt.Sprintf("  ✉ %d", n))
		}
		if n := m.coord.ClaimCount(sess.ID); n > 0 {
			label += s.Faint.Render(fmt.Sprintf("  ⊙ %d", n))
		}
		// A spawned review is the one thing here that costs money while
		// nobody is watching it: it has no pane, and the session that asked
		// for it has moved on. The badge is what makes it visible, and the
		// figure stays after the last one finishes so a total is not lost the
		// moment it stops moving.
		//
		// Coloured by the same rule as the two above. A run in flight is
		// spending now, which is the accent's job; a settled total is a fact
		// about the past, like a claim.
		if n, spent := m.coord.Analyses(sess.ID); n > 0 || spent > 0 {
			if n > 0 {
				label += s.Accent.Render(fmt.Sprintf("  ⚗ %d · $%.2f", n, spent))
			} else {
				label += s.Faint.Render(fmt.Sprintf("  ⚗ $%.2f", spent))
			}
		}
	}

	// The status line is truncated like the other two. It is the one that grows
	// without a bound the caller controls — "Needs you" plus two badges with
	// two-digit counts passes 24 columns, and an over-long line does not widen
	// the sidebar, it wraps: the card becomes four rows, the remainder starts
	// at column 0 with no bar, and the column is a row taller than the pane
	// beside it.
	return []string{
		bar + num + titleStyle.Render(truncate(title, width-3-numW)),
		bar + s.Faint.Render(truncate("⑂ "+ref, width-3)),
		bar + truncate(style.Render(glyph+" "+label), width-3),
	}
}
