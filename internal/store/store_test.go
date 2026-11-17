package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain points the state directory somewhere disposable for every test in
// the package, because this is the package that decides where Save writes. A
// test that forgets t.Setenv writes the user's own state.json instead, which
// is how a passing suite deleted a developer's registered projects.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "deck-store-state")
	if err != nil {
		fmt.Fprintln(os.Stderr, "test state dir:", err)
		os.Exit(1)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := &State{}
	p := s.AddProject(Project{Name: "medvault", Path: "/repos/medvault"})
	s.AddSession(Session{
		ProjectID: p.ID,
		Title:     "add transcript auto-generation",
		Name:      "scheming-hawk-jhgk",
		Branch:    "session/scheming-hawk-jhgk",
		Dir:       "/worktrees/scheming-hawk-jhgk",
		Isolated:  true,
	})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Projects) != 1 || len(got.Sessions) != 1 {
		t.Fatalf("loaded %d projects and %d sessions, want 1 and 1",
			len(got.Projects), len(got.Sessions))
	}
	if got.Sessions[0].Branch != "session/scheming-hawk-jhgk" {
		t.Errorf("branch = %q", got.Sessions[0].Branch)
	}
	if !got.Sessions[0].Isolated {
		t.Error("isolated flag did not survive the round trip")
	}
}

// TestLoadMissingIsEmpty covers the first run: no state file yet.
func TestLoadMissingIsEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s.Projects) != 0 {
		t.Errorf("fresh store has %d projects, want 0", len(s.Projects))
	}
}

// TestLoadCorruptFails is the important half. A state file that exists but
// does not parse means sessions we cannot see; starting empty would let
// Deck create duplicate worktrees over the top of them.
func TestLoadCorruptFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "deck", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("corrupt state loaded without error")
	}
}

func TestSessionsForIsNewestFirst(t *testing.T) {
	s := &State{}
	p := s.AddProject(Project{Name: "demo", Path: "/demo"})
	now := time.Now()
	s.AddSession(Session{ProjectID: p.ID, Name: "older", CreatedAt: now.Add(-time.Hour)})
	s.AddSession(Session{ProjectID: p.ID, Name: "newer", CreatedAt: now})
	s.AddSession(Session{ProjectID: "other", Name: "elsewhere", CreatedAt: now})

	got := s.SessionsFor(p.ID)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}
	if got[0].Name != "newer" {
		t.Errorf("first session = %q, want newer", got[0].Name)
	}
}

// names lists a project's sessions by name, in the order SessionsFor gives.
func names(s *State, projectID string) string {
	var out []string
	for _, sess := range s.SessionsFor(projectID) {
		out = append(out, sess.Name)
	}
	return strings.Join(out, ",")
}

// TestMoveSessionReorders covers a move in each direction, one that changes
// nothing, and one past the end, which clamps rather than refusing.
func TestMoveSessionReorders(t *testing.T) {
	s := &State{}
	p := s.AddProject(Project{Name: "demo", Path: "/demo"})
	now := time.Now()
	for i, name := range []string{"a", "b", "c"} {
		s.AddSession(Session{ProjectID: p.ID, Name: name, CreatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	other := s.AddSession(Session{ProjectID: "other", Name: "elsewhere", CreatedAt: now})
	id := func(name string) string { return s.SessionsFor(p.ID)[strings.Index("abc", name)].ID }
	c, a := id("c"), id("a")

	if !s.MoveSession(c, 0) || names(s, p.ID) != "c,a,b" {
		t.Errorf("moving c to the top gave %s", names(s, p.ID))
	}
	if s.MoveSession(c, 0) {
		t.Error("a move to where the session already is reported a move")
	}
	if !s.MoveSession(a, 99) || names(s, p.ID) != "c,b,a" {
		t.Errorf("moving a past the end gave %s, want it clamped to the bottom", names(s, p.ID))
	}
	if got := s.Session(other.ID).Order; got != orderFor(now) {
		t.Errorf("a session of another project changed order: %d", got)
	}
}

// TestANewSessionLandsFirstAfterAMove keeps the newest-first rule once the
// user has arranged the list: a move only permutes the values given out.
func TestANewSessionLandsFirstAfterAMove(t *testing.T) {
	s := &State{}
	p := s.AddProject(Project{Name: "demo", Path: "/demo"})
	old := time.Now().Add(-time.Hour)
	s.AddSession(Session{ProjectID: p.ID, Name: "a", CreatedAt: old})
	b := s.AddSession(Session{ProjectID: p.ID, Name: "b", CreatedAt: old.Add(-time.Minute)})
	s.MoveSession(b.ID, 0)

	s.AddSession(Session{ProjectID: p.ID, Name: "new"})
	if got := names(s, p.ID); got != "new,b,a" {
		t.Errorf("order = %s, want the new session first and the move kept", got)
	}
}

// TestMoveSessionWithEqualOrders is a hand-edited file whose sessions share a
// value. Reassigning equal values would move nothing and still say it moved.
func TestMoveSessionWithEqualOrders(t *testing.T) {
	s := &State{Sessions: []Session{
		{ID: "1", ProjectID: "p", Name: "a", Order: 5},
		{ID: "2", ProjectID: "p", Name: "b", Order: 5},
	}}
	if !s.MoveSession("2", 0) || names(s, "p") != "b,a" {
		t.Errorf("order = %s, want b moved first", names(s, "p"))
	}
}

// TestLoadKeepsTheOrderOfAnOlderFile: a file written before Order existed
// opens newest first, as it always did, and a move then survives a reload.
func TestLoadKeepsTheOrderOfAnOlderFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"projects":[{"id":"p","name":"demo","path":"/demo"}],"sessions":[
		{"id":"1","project_id":"p","name":"older","created_at":"2026-01-01T10:00:00Z"},
		{"id":"2","project_id":"p","name":"newer","created_at":"2026-01-02T10:00:00Z"}]}`
	path, err := statePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := names(s, "p"); got != "newer,older" {
		t.Fatalf("an older file opened as %s, want newest first", got)
	}
	s.MoveSession("1", 0)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := names(again, "p"); got != "older,newer" {
		t.Errorf("after a move and a reload the order is %s, want older,newer", got)
	}
}
