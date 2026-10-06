package ui

// Leaving Deck. Quitting stops every agent, because the agents are Deck's
// children and end with it. Under tmux there is a second way out: detaching
// leaves Deck, and so every agent, running for the next attach.

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/agent"
)

// The rows of the quit modal, as ids for the reason endClose is one: the
// label is what the user reads and the id is what quitPicked branches on.
const (
	quitStay = "stay"
	quitTmux = "tmux"
	quitQuit = "quit"
)

// detachClient detaches the tmux client that pressed the key. With no -t, tmux
// works out which client is in use. With two clients attached, the one that
// typed left and the other stayed. A variable so a test can see the call
// without detaching the terminal running the tests.
var detachClient = func() error {
	return exec.Command("tmux", "detach-client").Run()
}

// requestQuit is every quit key. It asks only when an agent is running,
// because that is the only thing quitting can lose.
//
// The cursor starts on the row that loses nothing. q and then a reflexive ↵
// is exactly the slip this modal exists to catch, so ↵ alone must not quit.
func (m Model) requestQuit() (tea.Model, tea.Cmd) {
	live := m.liveAgents()
	if live == 0 {
		return m, tea.Quit
	}

	safe := pickerRow{id: quitStay, label: "Stay", desc: "back to Deck"}
	if os.Getenv("TMUX") != "" {
		safe = pickerRow{id: quitTmux, label: "Detach tmux", desc: "agents keep running — tmux attach resumes"}
	}
	rows := []pickerRow{
		safe,
		{id: quitQuit, label: "Quit", desc: "stop " + plural(live, "running agent")},
	}
	m.picker = newPicker(pickQuit, "Quit Deck", rows, safe.id)
	return m, nil
}

// quitPicked carries out the choice the modal collected.
func (m Model) quitPicked(choice string) (tea.Model, tea.Cmd) {
	switch choice {
	case quitQuit:
		return m, tea.Quit
	case quitTmux:
		if err := detachClient(); err != nil {
			m.fault = fmt.Errorf("tmux detach-client: %w", err)
		}
	}
	return m, nil
}

// liveAgents counts the runners whose process has not exited. An exited
// runner stays in the map until its session is restarted or closed, and
// quitting cannot lose it.
func (m Model) liveAgents() int {
	n := 0
	for _, r := range m.runners {
		if r.Status() != agent.Exited {
			n++
		}
	}
	return n
}
