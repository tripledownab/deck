package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
