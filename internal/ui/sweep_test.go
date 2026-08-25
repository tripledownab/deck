package ui

import (
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/store"
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
