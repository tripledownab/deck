package ui

// The list modal — a scrolling, filterable column of rows over the frame.
// One widget, two jobs; pickerctl.go decides what each job means.

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pickerKind says what a selection means. The picker is a list widget; only
// the caller knows whether a row is a theme to apply or a project to assign.
type pickerKind int

const (
	pickTheme pickerKind = iota
	pickProject
)

// picker is a modal list: a title, rows, a cursor.
//
// It exists rather than reusing the form's choice field because that field
// lays its options out as a row of chips and falls back to a one-at-a-time
// cycler when they overflow. Cycling is fine for two working-copy options and
// poor for twelve themes, where you want to see the list and move down it.
type picker struct {
	kind  pickerKind
	title string
	rows  []pickerRow
	index int

	// restore is what was active when the picker opened, put back on cancel.
	//
	// It has to be captured here rather than read from the model on the
	// cancelling keypress: previewing overwrites the model's current value on
	// every cursor move, so by the time esc arrives the model no longer knows
	// what the user started with.
	restore string
}

type pickerRow struct {
	id, label, desc string
}

func newPicker(kind pickerKind, title string, rows []pickerRow, selected string) *picker {
	p := &picker{kind: kind, title: title, rows: rows, restore: selected}
	for i, r := range rows {
		if r.id == selected {
			p.index = i
			break
		}
	}
	return p
}

// selected is the id under the cursor.
func (p *picker) selected() string {
	if p.index < 0 || p.index >= len(p.rows) {
		return ""
	}
	return p.rows[p.index].id
}

// update moves the cursor. commit is true on Enter, cancel on Esc. The caller
// reads selected() after every key so it can preview as the cursor moves.
func (p *picker) update(msg tea.KeyMsg) (commit, cancel bool) {
	switch {
	case msg.Type == tea.KeyEsc:
		return false, true
	case msg.Type == tea.KeyEnter:
		return true, false
	case msg.Type == tea.KeyUp, isRune(msg, 'k'):
		if p.index > 0 {
			p.index--
		}
	case msg.Type == tea.KeyDown, isRune(msg, 'j'):
		if p.index < len(p.rows)-1 {
			p.index++
		}
	case msg.Type == tea.KeyHome:
		p.index = 0
	case msg.Type == tea.KeyEnd:
		p.index = len(p.rows) - 1
	}
	return false, false
}

func (p *picker) view(s styleSet, width, height int) string {
	boxWidth := min(width-8, 62)
	if boxWidth < 30 {
		boxWidth = max(width-4, 20)
	}
	inner := boxWidth - 4

	var b strings.Builder
	b.WriteString(s.Title.Render(p.title) + "\n")
	b.WriteString(s.Rule.Render(strings.Repeat("─", inner)) + "\n\n")

	// Room for the title, rule, blank, footer and the modal's own padding.
	visible := clamp(height-10, 3, len(p.rows))
	for _, i := range windowIndexes(len(p.rows), p.index, visible) {
		r := p.rows[i]
		marker, style := "  ", s.Muted
		if i == p.index {
			marker, style = s.Accent.Render("▸ "), s.Value.Bold(true)
		}
		line := marker + style.Render(r.label)
		if r.desc != "" {
			line += s.Faint.Render("  " + r.desc)
		}
		b.WriteString(truncateStyled(line, inner) + "\n")
	}

	b.WriteString("\n" + s.Footer.Render("↑/↓ move · ↵ apply · esc cancel"))
	box := s.Modal.Width(boxWidth).Padding(1, 2).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// windowIndexes returns the indexes to draw so the cursor stays visible,
// scrolling only when it has to.
func windowIndexes(total, focus, visible int) []int {
	if visible >= total {
		visible = total
	}
	start := focus - visible/2
	if start < 0 {
		start = 0
	}
	if start > total-visible {
		start = total - visible
	}
	out := make([]int, 0, visible)
	for i := start; i < start+visible; i++ {
		out = append(out, i)
	}
	return out
}

// themeRows is the picker's content for the theme list.
func themeRows() []pickerRow {
	rows := make([]pickerRow, 0, len(themes))
	for _, t := range themes {
		rows = append(rows, pickerRow{id: t.id, label: t.label, desc: t.desc})
	}
	return rows
}
