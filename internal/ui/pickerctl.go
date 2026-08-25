package ui

// Driving the list modal: opening it for each kind it serves, and routing
// its keys. picker.go is the widget; this is the part that knows what the
// two kinds mean — a theme previews by applying, a form field does not.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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

// pickerKey drives the theme picker.
//
// The palette is applied as the cursor moves, so the whole frame behind the
// modal restyles live and you judge a theme by the app rather than by its
// name. Esc puts back what was active before, which is why the id is captured
// on entry rather than read back from settings.
func (m Model) pickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	commit, cancel := m.picker.update(msg)

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
