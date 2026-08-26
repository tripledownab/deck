package ui

import (
	"testing"
	"time"

	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/store"
	"os"
	"path/filepath"
)

// TestSweepReleasesSelfExitedSession is the regression for the gap that the
// coordinator's own tests could not see.
//
// Unregistering was wired only to the two deliberate paths — stop the agent,
// close the session. An agent that left on its own stayed registered forever,
// so its claims outlived it and every sibling was told a dead process owned
// those files. coord.TestUnregisterReleasesClaims passed throughout, because
// it calls Unregister directly: it proves the coordinator releases when told,
// not that anything tells it.
func TestSweepReleasesSelfExitedSession(t *testing.T) {
	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "work", Dir: t.TempDir(),
	})

	m := New(st, "bash", nil).WithCoordinator(c)

	// An agent that exits by itself, the way /exit or a crash would.
	r := startAgent(t, sess.Dir, "exit 0")
	m.runners[sess.ID] = r
	c.Register(coord.Session{ID: sess.ID, ProjectID: p.ID, Name: sess.Name, Dir: sess.Dir})
	c.Claim(sess.ID, []string{"src/parser.go"}, "rewriting")

	waitExited(t, r)

	// Before the sweep the coordinator still believes the session is live.
	if len(c.Registered()) != 1 {
		t.Fatalf("registered = %v, want the session still listed", c.Registered())
	}

	// Driven through Update, not by calling sweepExited: the sweep being
	// correct is worth nothing if nothing invokes it, and that was the actual
	// defect.
	m.Update(frameMsg(time.Now()))

	if ids := c.Registered(); len(ids) != 0 {
		t.Errorf("registered = %v after a frame tick, want empty", ids)
	}
	if n := c.ClaimCount(sess.ID); n != 0 {
		t.Errorf("dead session still holds %d claims", n)
	}
}

// TestSweepKeepsLiveSessions is the other half: a running agent must survive
// the sweep, or coordination would clear itself every frame.
func TestSweepKeepsLiveSessions(t *testing.T) {
	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{ProjectID: p.ID, Name: "wily-crane-bbbb", Dir: t.TempDir()})

	m := New(st, "bash", nil).WithCoordinator(c)

	m.runners[sess.ID] = startAgent(t, sess.Dir, "")
	c.Register(coord.Session{ID: sess.ID, ProjectID: p.ID, Name: sess.Name, Dir: sess.Dir})

	m.Update(frameMsg(time.Now()))

	if ids := c.Registered(); len(ids) != 1 {
		t.Errorf("registered = %v, want the live session kept", ids)
	}
}

// TestSweepReleasesSessionWithNoRunner covers the other way a session can be
// stranded: its runner removed from the map while the registration survived.
func TestSweepReleasesSessionWithNoRunner(t *testing.T) {
	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	m := New(&store.State{}, "bash", nil).WithCoordinator(c)
	c.Register(coord.Session{ID: "orphan", ProjectID: "p1", Name: "orphan-one", Dir: t.TempDir()})

	m.Update(frameMsg(time.Now()))

	if ids := c.Registered(); len(ids) != 0 {
		t.Errorf("registered = %v, want the orphan released", ids)
	}
}

// TestExitedAgentReleasesTheKeyboard is the reason pressing esc in a session
// felt like a dead end. landOn drops the attachment when the cursor moves to a
// session with no process; nothing covered the case where the cursor stays and
// the process leaves. Every key then went to a dead PTY, and the first one to
// be refused reported "send to agent: process has exited" — a fault in Deck,
// as far as the reader could tell, rather than the agent having quit.
func TestExitedAgentReleasesTheKeyboard(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "work",
		Dir: t.TempDir(), Agent: "bash",
	})

	m := New(st, "bash", nil)
	m.width, m.height = 100, 30
	m.screen = screenSession
	m.attached = true

	r := startAgent(t, sess.Dir, "exit 0")
	m.runners[sess.ID] = r
	waitExited(t, r)

	if !m.attached {
		t.Fatal("the fixture is not attached, so it proves nothing")
	}

	// Driven through Update rather than by calling the sweep directly: the
	// release being correct is worth nothing if no tick invokes it.
	next, _ := m.Update(frameMsg(time.Now()))
	m = next.(Model)

	if m.attached {
		t.Error("still attached to an agent that exited; every key goes to a dead PTY")
	}

	// And now the chrome key that restarts it actually reaches a handler.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model).runners[sess.ID]; got == r {
		t.Error("enter did not start a new process; the session is unrecoverable")
	}
}

// TestExitedPaneSaysHowToRestart pins the hint. The banner reported what had
// happened and nothing said what to do about it.
func TestExitedPaneSaysHowToRestart(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "wily-crane-bbbb", Title: "work",
		Dir: t.TempDir(), Agent: "bash",
	})

	m := New(st, "bash", nil)
	m.width, m.height = 100, 30
	m.screen = screenSession

	r := startAgent(t, sess.Dir, "exit 0")
	m.runners[sess.ID] = r
	waitExited(t, r)

	pane := m.renderPane(80, 12)
	if !strings.Contains(pane, "the agent exited") {
		t.Errorf("pane does not report the exit:\n%s", pane)
	}
	if !strings.Contains(pane, "to start bash again") {
		t.Errorf("pane does not say how to restart:\n%s", pane)
	}
}

// TestWillResumeOnlyForIsolatedSessions guards the promise in that hint. A
// session in the project directory starts fresh, because another tool may have
// been the last to run there, so claiming a resume would be a lie.
func TestWillResumeOnlyForIsolatedSessions(t *testing.T) {
	shared := store.Session{Dir: t.TempDir(), Isolated: false}
	if willResume(&shared) {
		t.Error("a session in the project directory claims it will resume")
	}
}

// TestWillResumeIsClaudeOnly guards a flag from reaching an agent that does
// not have it. dirHasHistory reads claude's transcript directory, so a
// worktree claude once used would otherwise hand --continue to whatever ran
// there next, and the pane would promise a resume that cannot happen.
func TestWillResumeIsClaudeOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects", claudeSlug(dir)), 0o755); err != nil {
		t.Fatal(err)
	}

	claude := store.Session{Dir: dir, Isolated: true, Agent: "claude"}
	if !willResume(&claude) {
		t.Error("an isolated claude session with a transcript does not resume")
	}
	for _, other := range []string{"codex", "gemini", "cathode"} {
		sess := store.Session{Dir: dir, Isolated: true, Agent: other}
		if willResume(&sess) {
			t.Errorf("%s claims it will resume; --continue is claude's flag", other)
		}
	}
}
