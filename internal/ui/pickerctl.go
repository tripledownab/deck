package ui

// Driving the list modal: opening it for each kind it serves, and routing
// its keys. picker.go is the widget; this is the part that knows what the
// two kinds mean — a theme previews by applying, a form field does not.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// The two rows of the end-session modal. They are ids rather than labels
// because the label is what the user reads and the id is what endPicked
// branches on, and a row renamed for clarity must not change what it does.
const (
	endClose  = "close"
	endDelete = "delete"
)

// openThemePicker opens the theme list, positioned on the active theme.
func (m Model) openThemePicker() (tea.Model, tea.Cmd) {
	m.picker = newPicker(pickTheme, "Theme", themeRows(), m.theme)
	return m, nil
}

// openFieldPicker opens the full list for the form field under the cursor.
//
// The project field cycles with ←/→, which is fine for the two or three
// projects a new user has and useless once the list grows with the machine.
// The picker is the same widget the theme list uses: a window that scrolls,
// rather than a value you step past.
func (m Model) openFieldPicker() (tea.Model, tea.Cmd) {
	fl := &m.form.fields[m.form.index]
	rows := make([]pickerRow, 0, len(fl.choices))
	for _, c := range fl.choices {
		rows = append(rows, pickerRow{id: c.value, label: c.label, desc: c.help})
	}
	m.picker = newPicker(pickProject, fl.label, rows, fl.value())
	return m, nil
}

// openEndSessionPicker asks what ending a session should do with the worktree
// behind it.
//
// Close is the row the cursor starts on. It is the reversible outcome, and a
// modal that opens on the destructive one turns a confirmation into a trap.
//
// The Delete row carries what that session changed rather than a warning about
// what it is about to lose. A confirmation that states a fact can be answered;
// one that states a warning can only be believed or dismissed.
func (m Model) openEndSessionPicker(sess *store.Session) (tea.Model, tea.Cmd) {
	rows := []pickerRow{
		{id: endClose, label: "Close", desc: "keep the worktree at " + sess.Dir},
		{id: endDelete, label: "Delete", desc: "remove the worktree and branch — " + workSummary(*sess)},
	}
	m.pickerSubject = sess.ID
	m.picker = newPicker(pickEndSession, "End "+sessionLabel(*sess), rows, endClose)
	return m, nil
}

// pickerKey drives the theme picker.
//
// The palette is applied as the cursor moves, so the whole frame behind the
// modal restyles live and you judge a theme by the app rather than by its
// name. Esc puts back what was active before, which is why the id is captured
// on entry rather than read back from settings.
func (m Model) pickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	commit, cancel := m.picker.update(msg)

	// Connecting previews nothing: a link is a change to the store, and
	// applying one as the cursor moves would write a record for every session
	// scrolled past. So it acts on commit only, and cancel just closes.
	if m.picker.kind == pickConnect {
		switch {
		case cancel:
			m.picker = nil
		case commit:
			id := m.picker.selected()
			m.picker = nil
			return m.connectPicked(id)
		}
		return m, nil
	}

	// Ending previews nothing, and esc is the way out of a question rather than
	// a way to undo an answer. The modal exists so the destructive row costs a
	// second, deliberate keystroke.
	if m.picker.kind == pickEndSession {
		switch {
		case cancel:
			m.picker = nil
		case commit:
			choice := m.picker.selected()
			m.picker = nil
			return m.endPicked(choice)
		}
		return m, nil
	}

	// A field picker floats over the open form and only writes back into it.
	// Nothing is previewed and nothing is persisted, so cancel is simply
	// closing it.
	if m.picker.kind == pickProject {
		switch {
		case cancel:
			m.picker = nil
		case commit:
			id := m.picker.selected()
			m.picker = nil
			fl := &m.form.fields[m.form.index]
			for i, c := range fl.choices {
				if c.value == id {
					fl.selected = i
					break
				}
			}
		}
		return m, nil
	}

	switch {
	case cancel:
		restore := m.picker.restore
		m.picker = nil
		return m.WithTheme(restore), nil

	case commit:
		id := m.picker.selected()
		m.picker = nil
		m = m.WithTheme(id)
		m.settings.Theme = id
		if err := m.settings.Save(); err != nil {
			m.fault = fmt.Errorf("save theme: %w", err)
			return m, nil
		}
		m.notice = "theme: " + themeLabel(id)
		return m, nil
	}

	// Preview whatever the cursor is on now.
	return m.WithTheme(m.picker.selected()), nil
}
