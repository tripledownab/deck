package ui

// Keystroke routing. Everything here decides *who* receives a key — a modal,
// the agent pane, or the chrome — and delegates the work itself to actions.go.
// The mouse is routed in mouse.go.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A key between two clicks makes them two first clicks, not a pair.
	m.lastPress = target{}

	// A modal owns every key while it is up.
	if m.picker != nil {
		return m.pickerKey(msg)
	}
	if m.browser != nil {
		return m.browserKey(msg)
	}
	if m.form != nil {
		cmd, submit, cancel, pick := m.form.update(msg)
		switch {
		case cancel:
			m.form = nil
		case submit:
			return m.commitForm()
		case pick:
			return m.openFieldPicker()
		}
		return m, cmd
	}

	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// The prefix is armed: this key is a command, wherever we are.
	if m.armed {
		m.armed = false
		return m.command(msg)
	}
	if msg.String() == PrefixKey {
		m.armed = true
		return m, nil
	}

	// Attached: every remaining key belongs to the agent.
	if m.screen == screenSession && m.attached {
		if b := keyToBytes(msg); len(b) > 0 {
			m.sendToAgent(b)
		}
		return m, nil
	}

	if m.screen == screenDashboard {
		return m.dashboardKey(msg)
	}
	return m.sessionKey(msg)
}

// sendToAgent writes to the attached agent. Every path that types into the
// pane comes through here, so none of them can drop a failed write: it is
// reported, and the attachment goes with it.
func (m *Model) sendToAgent(b []byte) {
	r := m.currentRunner()
	if r == nil || !m.attached {
		return
	}
	if err := r.Write(b); err != nil {
		m.fault = fmt.Errorf("send to agent: %w", err)
		m.attached = false
	}
}

// command handles the key pressed after the prefix.
func (m Model) command(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Prefix twice sends a literal prefix through to the agent.
	if msg.String() == PrefixKey {
		m.sendToAgent([]byte{0x07})
		return m, nil
	}

	switch msg.String() {
	case "d":
		m.screen = screenDashboard
		m.attached = false
	case "s":
		if len(m.rows) > 0 {
			m.screen = screenSession
		}
	case "n":
		return m.openNewSessionForm()
	case "j", "down":
		m.moveRow(1)
	case "k", "up":
		m.moveRow(-1)
	case "x":
		m.stopCurrent()
	case "esc", " ":
		m.attached = false
	case "enter", "i":
		return m.attach()
	case "t":
		return m.openThemePicker()
	case "?":
		m.showHelp = true
	case "q":
		return m, tea.Quit
	default:
		// ^g 1…9 jumps straight to a session. Only on a successful jump does
		// the screen follow: an out-of-range digit leaves you where you are,
		// with the notice selectSession set.
		if n, ok := digitKey(msg); ok && m.jumpToSession(n) {
			m.screen = screenSession
		}
	}
	return m, nil
}

// digitKey reads a single 1…9 keypress.
//
// The length check is not defensive padding. Bubble Tea batches the runes of
// one read into a single KeyRunes message, so a fast or pasted "12" arrives as
// one message whose Runes are both digits — and taking Runes[0] would jump to
// session 1 on input that meant nothing of the kind. There is no session 0, so
// the range starts at 1.
func digitKey(msg tea.KeyMsg) (int, bool) {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return 0, false
	}
	r := msg.Runes[0]
	if r < '1' || r > '9' {
		return 0, false
	}
	return int(r - '0'), true
}
