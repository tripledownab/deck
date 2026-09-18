package ui

// Deleting a session against a real repository. The refusals are the point of
// the feature, so they are driven through git rather than a stub: a fake that
// refuses when told to would prove only that it was told.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tripledownab/deck/internal/gittest"
	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// deckWithWorktree builds a model holding one project that is a real
// repository and one isolated session with a real worktree checked out of it.
func deckWithWorktree(t *testing.T) (Model, store.Session) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	repo, head := gittest.RepoWith(t, "a.txt", "one\n")
	dest := filepath.Join(t.TempDir(), "wt")
	const branch = "session/quiet-otter-abcd"
	if err := gitx.AddWorktree(repo, dest, branch); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: repo})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID,
		Name:      "quiet-otter-abcd",
		Title:     "work",
		Branch:    branch,
		BaseRef:   head,
		Dir:       dest,
		Isolated:  true,
	})
	m := New(st, "bash", nil)
	m.focusContent()
	return m, *sess
}

// TestDeleteRemovesTheWorktreeAndBranch is the outcome the feature exists for:
// a session that changed nothing leaves nothing behind.
func TestDeleteRemovesTheWorktreeAndBranch(t *testing.T) {
	m, sess := deckWithWorktree(t)
	repo := m.state.Project(sess.ProjectID).Path

	m.deleteSession(sess)

	if m.fault != nil {
		t.Fatalf("deleting a clean session reported: %v", m.fault)
	}
	if _, err := os.Stat(sess.Dir); !os.IsNotExist(err) {
		t.Errorf("the worktree survived: stat err = %v", err)
	}
	if out := gittest.Run(t, repo, "branch", "--list", sess.Branch); out != "" {
		t.Errorf("branch --list = %q, want the branch gone", out)
	}
	if n := len(m.state.Sessions); n != 0 {
		t.Errorf("sessions = %d, want the record forgotten", n)
	}
}

// TestDeleteRefusesADirtyWorktree is the promise that nothing is forced. The
// file is never added, because untracked is the case that catches people: an
// agent that wrote one and stopped has left work with nothing in `git diff` to
// show for it.
func TestDeleteRefusesADirtyWorktree(t *testing.T) {
	m, sess := deckWithWorktree(t)
	if err := os.WriteFile(filepath.Join(sess.Dir, "scratch.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m.deleteSession(sess)

	if m.fault == nil {
		t.Fatal("deleting a tree holding work reported nothing")
	}
	if _, err := os.Stat(sess.Dir); err != nil {
		t.Errorf("a refused delete still took the directory: %v", err)
	}
	// The session has to survive its own refused delete. A record dropped over
	// a surviving worktree is an orphan the user can no longer see or retry.
	if n := len(m.state.Sessions); n != 1 {
		t.Errorf("sessions = %d after a refused delete, want the session kept", n)
	}
}

// TestDeleteKeepsAnUnmergedBranch is the worktree-is-the-gate rule. A session
// that committed leaves a clean tree, so the tree goes and the branch is then
// the only copy of the work. git refuses it, the session still goes, and the
// notice is the only place the user learns where the work is.
func TestDeleteKeepsAnUnmergedBranch(t *testing.T) {
	m, sess := deckWithWorktree(t)
	repo := m.state.Project(sess.ProjectID).Path
	if err := os.WriteFile(filepath.Join(sess.Dir, "b.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, sess.Dir, "add", "b.txt")
	gittest.Run(t, sess.Dir, "commit", "-q", "-m", "session work")

	m.deleteSession(sess)

	if m.fault != nil {
		t.Fatalf("a committed session was refused: %v", m.fault)
	}
	if _, err := os.Stat(sess.Dir); !os.IsNotExist(err) {
		t.Errorf("the worktree survived: stat err = %v", err)
	}
	if out := gittest.Run(t, repo, "branch", "--list", sess.Branch); out == "" {
		t.Error("the branch holding the only copy of the work was deleted")
	}
	if n := len(m.state.Sessions); n != 0 {
		t.Errorf("sessions = %d, want the record forgotten", n)
	}
	if !strings.Contains(m.notice, sess.Branch) {
		t.Errorf("notice = %q, want it to name the branch that was kept", m.notice)
	}
}

// TestDeleteRefusesASessionWithoutAWorktree guards the case the modal is not
// supposed to reach. A non-isolated session's Dir is the project directory, so
// a delete that got here would aim git at the user's own checkout.
func TestDeleteRefusesASessionWithoutAWorktree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo, _ := gittest.RepoWith(t, "a.txt", "one\n")

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: repo})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "inplace", Title: "in place", Dir: repo,
	})
	m := New(st, "bash", nil)

	m.deleteSession(*sess)

	if _, err := os.Stat(filepath.Join(repo, "a.txt")); err != nil {
		t.Fatalf("the project's own checkout was touched: %v", err)
	}
	if n := len(m.state.Sessions); n != 1 {
		t.Errorf("sessions = %d, want the session kept", n)
	}
	if m.notice == "" {
		t.Error("the refusal said nothing about why")
	}
}

// TestWorkSummaryIsOneLine keeps the Delete row answerable. gitx.Work.Stat is
// one line per changed file followed by the total, and only the total fits.
func TestWorkSummaryIsOneLine(t *testing.T) {
	_, sess := deckWithWorktree(t)
	for _, name := range []string{"b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(sess.Dir, name), []byte("work\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := workSummary(sess)

	if strings.Contains(got, "\n") {
		t.Errorf("summary spans lines, so the row cannot hold it:\n%s", got)
	}
	if !strings.Contains(got, "2 files changed") {
		t.Errorf("summary = %q, want the total across both new files", got)
	}
}

// TestWorkSummaryReportsAFailedMeasurement covers a session with no recorded
// base. Answering "no files changed" there would be a confirmation stating a
// fact it does not have, which is worse than one admitting it could not look.
func TestWorkSummaryReportsAFailedMeasurement(t *testing.T) {
	_, sess := deckWithWorktree(t)
	sess.BaseRef = ""

	got := workSummary(sess)

	if strings.Contains(got, "no files changed") {
		t.Errorf("summary = %q, want it to admit the measurement failed", got)
	}
	if !strings.Contains(got, "could not measure") {
		t.Errorf("summary = %q, want it to say the measurement failed", got)
	}
}
