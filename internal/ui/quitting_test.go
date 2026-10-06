package ui

// The quit modal: when it asks, which row ↵ lands on, and what each row does.
// Driven through handleKey, because the bug it fixes was a key, not a function.

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// deckWithLiveAgent is a model on the dashboard with one running bash agent.
func deckWithLiveAgent(t *testing.T) Model {
	t.Helper()
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{ProjectID: p.ID, Name: "n", Title: "t", Dir: t.TempDir(), Agent: "bash"})
	m := New(st, "bash", nil)
	m.runners[sess.ID] = startAgent(t, sess.Dir, "")
	return m
}

func typeKey(t *testing.T, m Model, k tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.handleKey(k)
	return next.(Model), cmd
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

var (
	keyQ     = typed("q")
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
)

// TestQuitWithNothingRunningDoesNotAsk: with no agent to lose there is no
// question to ask.
func TestQuitWithNothingRunningDoesNotAsk(t *testing.T) {
	_, cmd := typeKey(t, New(&store.State{}, "bash", nil), keyQ)
	if !quits(cmd) {
		t.Fatal("q with no agents running did not quit")
	}
}

// TestQuitAsksAndEnterStays is the slip the modal catches: q then a reflexive
// ↵ must leave every agent running. Both screens route q themselves, so both
// are driven, and esc must close the question without quitting too.
func TestQuitAsksAndEnterStays(t *testing.T) {
	t.Setenv("TMUX", "")
	base := deckWithLiveAgent(t)
	onSession := base
	onSession.screen = screenSession

	for name, m := range map[string]Model{"dashboard": base, "session": onSession} {
		for _, k := range []tea.KeyMsg{keyQ, {Type: tea.KeyCtrlC}} {
			for _, answer := range []tea.KeyMsg{keyEnter, {Type: tea.KeyEsc}} {
				m, cmd := typeKey(t, m, k)
				if quits(cmd) || m.picker == nil || m.picker.kind != pickQuit {
					t.Fatalf("%s: %s with an agent running did not open the quit modal", name, k)
				}
				m, cmd = typeKey(t, m, answer)
				if quits(cmd) || m.picker != nil {
					t.Fatalf("%s: %s then %s quit or left the modal up", name, k, answer)
				}
			}
		}
	}
}

// TestPrefixQuitAsks: ^g q is the quit key that works while attached, so it is
// the one most likely pressed with an agent mid-turn.
func TestPrefixQuitAsks(t *testing.T) {
	m := deckWithLiveAgent(t)
	m, _ = typeKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlG})
	m, cmd := typeKey(t, m, keyQ)
	if quits(cmd) || m.picker == nil || m.picker.kind != pickQuit {
		t.Fatal("^g q with an agent running did not open the quit modal")
	}
}

// TestExitedAgentsDoNotAsk: an agent that has already exited cannot be lost.
func TestExitedAgentsDoNotAsk(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{ProjectID: p.ID, Name: "n", Title: "t", Dir: t.TempDir(), Agent: "bash"})
	m := New(st, "bash", nil)
	r := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, r)
	m.runners[sess.ID] = r

	if _, cmd := typeKey(t, m, keyQ); !quits(cmd) {
		t.Fatal("q with only an exited agent asked instead of quitting")
	}
}

// TestQuitRowQuits: the second row is the way out, one deliberate key away.
func TestQuitRowQuits(t *testing.T) {
	t.Setenv("TMUX", "")
	m, _ := typeKey(t, deckWithLiveAgent(t), keyQ)
	m, _ = typeKey(t, m, keyDown)
	if _, cmd := typeKey(t, m, keyEnter); !quits(cmd) {
		t.Fatal("choosing Quit did not quit")
	}
}

// TestDetachUnderTmux: inside tmux the safe row detaches the client, and Deck
// itself keeps running.
func TestDetachUnderTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	called := 0
	orig := detachClient
	detachClient = func() error { called++; return nil }
	t.Cleanup(func() { detachClient = orig })

	m, _ := typeKey(t, deckWithLiveAgent(t), keyQ)
	if got := m.picker.selected(); got != quitTmux {
		t.Fatalf("cursor opened on %q, want %q", got, quitTmux)
	}
	m, cmd := typeKey(t, m, keyEnter)
	if quits(cmd) {
		t.Fatal("Detach tmux quit Deck, which stops every agent")
	}
	if called != 1 {
		t.Fatalf("detach-client ran %d times, want 1", called)
	}
	if m.fault != nil {
		t.Fatalf("a successful detach reported %v", m.fault)
	}
}

// TestDetachFailureIsReported: a tmux that refuses is said, not swallowed.
func TestDetachFailureIsReported(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	orig := detachClient
	detachClient = func() error { return errors.New("no current client") }
	t.Cleanup(func() { detachClient = orig })

	m, _ := typeKey(t, deckWithLiveAgent(t), keyQ)
	m, _ = typeKey(t, m, keyEnter)
	if m.fault == nil {
		t.Fatal("a failed detach reported nothing")
	}
}
