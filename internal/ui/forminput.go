package ui

// Keystroke handling for the form modal: focus, choice stepping, and the
// rune-at-a-time text path that keeps a binding from eating a typed word.

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (f *form) focus(i int) {
	for j := range f.fields {
		if f.fields[j].kind == fieldText {
			f.fields[j].input.Blur()
		}
	}
	f.index = (i + len(f.fields)) % len(f.fields)
	if f.fields[f.index].kind == fieldText {
		f.fields[f.index].input.Focus()
	}
}

// submitted reports whether every required field has a value.
func (f *form) submitted() bool {
	for i := range f.fields {
		if f.fields[i].optional || f.fields[i].kind == fieldChoice {
			continue
		}
		if f.fields[i].value() == "" {
			f.problem = f.fields[i].label + " is required"
			f.focus(i)
			return false
		}
	}
	f.problem = ""
	return true
}

// update handles a key for the form. It returns submit=true when the user
// asked to commit, and cancel=true when they backed out.
//
// Every branch dispatches on msg.Type, never on msg.String(). For a typed
// rune those two disagree in a way that silently corrupts input: Bubble Tea
// batches the runes of one read into a single KeyRunes message, and its
// String() is just the text — so typing the word "up" is indistinguishable
// from pressing the up arrow. A title of "wire up the parser" moved focus
// mid-word and dropped the rest of the sentence. Keep text and key names
// apart.
func (f *form) update(msg tea.KeyMsg) (cmd tea.Cmd, submit, cancel, pick bool) {
	switch msg.Type {
	case tea.KeyEsc:
		return nil, false, true, false
	case tea.KeyCtrlS:
		return nil, f.submitted(), false, false
	case tea.KeyTab, tea.KeyDown:
		f.focus(f.index + 1)
		return nil, false, false, false
	case tea.KeyShiftTab, tea.KeyUp:
		f.focus(f.index - 1)
		return nil, false, false, false
	case tea.KeyEnter:
		if f.fields[f.index].pickable {
			return nil, false, false, true
		}
		if f.index == len(f.fields)-1 {
			return nil, f.submitted(), false, false
		}
		f.focus(f.index + 1)
		return nil, false, false, false
	}

	cur := &f.fields[f.index]
	if cur.kind == fieldChoice {
		switch {
		case msg.Type == tea.KeyLeft, isRune(msg, 'h'):
			cur.selected = (cur.selected - 1 + len(cur.choices)) % len(cur.choices)
		case msg.Type == tea.KeyRight, msg.Type == tea.KeySpace, isRune(msg, 'l'):
			cur.selected = (cur.selected + 1) % len(cur.choices)
		}
		return nil, false, false, false
	}

	return f.typeInto(cur, msg), false, false, false
}

// typeInto feeds a key to a text field one rune at a time.
//
// The splitting is not cosmetic. Bubble Tea batches every rune of one read
// into a single KeyRunes message, and bubbles/textinput matches its own
// bindings with key.Matches, which compares msg.String(). For a batch, that
// String() is the typed text — so a title containing "up" matches the
// PrevSuggestion binding and the whole batch is swallowed instead of
// inserted. "down", "end", "home", and "delete" collide the same way.
//
// A one-rune message stringifies to that character, and no binding is a bare
// printable character, so splitting removes the collision entirely rather
// than disabling the four bindings that happen to collide today.
func (f *form) typeInto(fl *field, msg tea.KeyMsg) tea.Cmd {
	if msg.Type != tea.KeyRunes || len(msg.Runes) <= 1 {
		var c tea.Cmd
		fl.input, c = fl.input.Update(msg)
		return c
	}
	var cmd tea.Cmd
	for _, r := range msg.Runes {
		fl.input, cmd = fl.input.Update(tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{r},
			Alt:   msg.Alt,
			Paste: msg.Paste,
		})
	}
	return cmd
}

// isRune reports whether msg is exactly the single typed character r.
func isRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}
