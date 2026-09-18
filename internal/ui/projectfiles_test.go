package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tripledownab/deck/internal/gittest"
	"github.com/tripledownab/deck/internal/gitx"
)

// projectAndWorktree builds a repository that ignores the instruction files,
// and a real worktree of it. That is the arrangement the feature is for, and
// only a real worktree shows what git does and does not carry across.
func projectAndWorktree(t *testing.T) (project, worktree string) {
	t.Helper()
	project, _ = gittest.RepoWith(t, "a.txt", "one\n")
	ignore := strings.Join(agentInstructions, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, project, "add", ".gitignore")
	gittest.Run(t, project, "commit", "-q", "-m", "ignore instructions")

	worktree = filepath.Join(t.TempDir(), "wt")
	if err := gitx.AddWorktree(project, worktree, "session/test"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	return project, worktree
}

// TestLinkProjectFilesCarriesAnIgnoredFile is the case the feature exists for,
// and it is this repository's own: CLAUDE.md is gitignored here, so a worktree
// gets none of the instructions the project directory has.
//
// The fixture commits the .gitignore and leaves CLAUDE.md untracked, because
// that is what makes git leave it behind. Writing the file without ignoring it
// would prove nothing.
func TestLinkProjectFilesCarriesAnIgnoredFile(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("build with make\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "CLAUDE.md")); err == nil {
		t.Fatal("git carried the ignored file, so this test proves nothing")
	}

	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(worktree, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("the session cannot read the instructions: %v", err)
	}
	if string(body) != "build with make\n" {
		t.Errorf("read %q through the link, want the project's own text", body)
	}
}

// TestLinkProjectFilesSharesOneCopy is why it is a link and not a copy. An edit
// made in one session has to be the edit every sibling reads, or the project
// has as many sets of instructions as it has sessions.
func TestLinkProjectFilesSharesOneCopy(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}

	// The session edits through the link.
	if err := os.WriteFile(filepath.Join(worktree, "CLAUDE.md"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(project, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "second\n" {
		t.Errorf("the project still reads %q, so the session edited a copy", body)
	}
}

// TestLinkProjectFilesLeavesTrackedFilesAlone covers a project whose
// instructions are committed. git already put them in the worktree, and
// replacing a checked-out file with a link would take it out of the tree the
// session is supposed to be working in.
func TestLinkProjectFilesLeavesTrackedFilesAlone(t *testing.T) {
	project, _ := projectAndWorktree(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// -f because the fixture ignores the name. A project that commits its
	// instructions is the arrangement under test, and the ignore rule is what
	// every other test here needs.
	gittest.Run(t, project, "add", "-f", "CLAUDE.md")
	gittest.Run(t, project, "commit", "-q", "-m", "track instructions")

	worktree := filepath.Join(t.TempDir(), "wt2")
	if err := gitx.AddWorktree(project, worktree, "session/second"); err != nil {
		t.Fatal(err)
	}

	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}

	info, err := os.Lstat(filepath.Join(worktree, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("a tracked file was replaced with a link to the project's copy")
	}
}

// TestLinkProjectFilesCarriesADirectory covers .claude, which holds settings,
// commands and skills rather than one file. Linking the directory is what makes
// a later addition inside it reach every session without Deck running again.
func TestLinkProjectFilesCarriesADirectory(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}

	// Added after the link, so this asserts the link and not a snapshot of it.
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, ".claude", "settings.json")); err != nil {
		t.Errorf("a file added to the project after linking is not visible: %v", err)
	}
}

// TestLinkProjectFilesOnAProjectWithNone is the common case and must be quiet.
// A repository with no instructions is not a failure, and reporting one would
// put an error in front of every session on such a project.
func TestLinkProjectFilesOnAProjectWithNone(t *testing.T) {
	project, worktree := projectAndWorktree(t)

	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("a project with no instructions reported: %v", err)
	}

	entries, err := os.ReadDir(worktree)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			t.Errorf("linked %q, which the project does not have", e.Name())
		}
	}
}

// TestLinkProjectFilesReportsAFailure keeps the rule that failures are reported
// rather than absorbed. A session that started without its instructions would
// behave differently from one in the project directory, and nothing on screen
// would explain why.
func TestLinkProjectFilesReportsAFailure(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A worktree nothing can be written into. Refusing the link is the only
	// outcome available, so what is under test is whether it is reported.
	if err := os.Chmod(worktree, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(worktree, 0o755) })

	if err := linkProjectFiles(project, worktree); err == nil {
		t.Fatal("a link that could not be made reported nothing")
	}
}

// TestLinkProjectFilesLeavesAnUnignoredFileBehind is the rule that keeps a
// machine path out of a commit.
//
// The link's target is absolute. A link git can see is one `git add -A` away
// from publishing the layout of this machine, and an agent running in the
// worktree is exactly the thing that runs `git add -A`. Being ignored is also
// what keeps the tree removable, which the rollback in newSession depends on.
func TestLinkProjectFilesLeavesAnUnignoredFileBehind(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	// Narrow the project's ignore rules so AGENTS.md is no longer covered.
	// check-ignore reads the working tree, so this needs no commit — and the
	// two files then differ only in whether git can see them.
	if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte("CLAUDE.md\n.claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if err := os.WriteFile(filepath.Join(project, name), []byte("inst\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(worktree, "AGENTS.md")); err == nil {
		t.Error("linked a file git does not ignore, so a machine path is now committable")
	}
	// The ignored one still travels, so what refused above was the ignore rule
	// and not linking that had stopped working.
	if _, err := os.Lstat(filepath.Join(worktree, "CLAUDE.md")); err != nil {
		t.Errorf("the ignored file did not travel: %v", err)
	}
}

// TestIgnoredLinksDoNotBlockRemoval is the measurement the rollback in
// newSession rests on. git refuses to remove a worktree holding untracked
// files, and a linked instruction file would be one of those if it were not
// ignored — so a half-finished link would strand the tree it was trying to
// unwind.
func TestIgnoredLinksDoNotBlockRemoval(t *testing.T) {
	project, worktree := projectAndWorktree(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("inst\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := linkProjectFiles(project, worktree); err != nil {
		t.Fatalf("linkProjectFiles: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(worktree, "CLAUDE.md")); err != nil {
		t.Fatalf("nothing was linked, so this proves nothing: %v", err)
	}

	if err := gitx.RemoveWorktree(project, worktree); err != nil {
		t.Fatalf("a linked instruction file blocked the rollback: %v", err)
	}
}
