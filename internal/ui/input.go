package ui

// Keystroke routing. Everything here decides *who* receives a key — a modal,
// the agent pane, or the chrome — and delegates the work itself to actions.go.
// The mouse is routed in mouse.go.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/agent"
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
	// A fresh prefix starts a fresh number, whatever path disarmed the last.
	if msg.String() == PrefixKey {
		m.armed, m.jumpDigits = true, 0
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

// sendToAgent writes bytes to the attached agent.
func (m *Model) sendToAgent(b []byte) {
	m.toAgent(func(r *agent.Runner) error { return r.Write(b) })
}

// toAgent runs send against the attached agent. Every path that sends input
// to the pane comes through here, so none of them can drop a failed write: it
// is reported, and the attachment goes with it.
func (m *Model) toAgent(send func(*agent.Runner) error) {
	r := m.currentRunner()
	if r == nil || !m.attached {
		return
	}
	if err := send(r); err != nil {
		m.fault = fmt.Errorf("send to agent: %w", err)
		m.attached = false
	}
}

// command handles the key pressed after the prefix.
func (m Model) command(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.jumpDigits > 0 {
		return m.continueJump(msg)
	}
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
		return m.requestQuit()
	default:
		// ^g and a number jumps straight to a session; jump.go reads it.
		if ds, ok := digitKey(msg); ok {
			return m.typeDigits(ds)
		}
	}
	return m, nil
}
