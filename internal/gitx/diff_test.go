package gitx

import (
	"github.com/tripledownab/deck/internal/gittest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffSeesUncommittedWork is the case that would mislead a reviewer worst.
// A session that has edited but not committed has still done work, and being
// told "no changes" would be wrong rather than merely incomplete.
func TestDiffSeesUncommittedWork(t *testing.T) {
	dir, base := gittest.RepoWith(t, "router.go", "package gateway\n")
	if err := os.WriteFile(filepath.Join(dir, "router.go"),
		[]byte("package gateway\n\nfunc Route() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := Diff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Patch, "func Route()") {
		t.Errorf("uncommitted change missing from the patch:\n%s", w.Patch)
	}
	if !strings.Contains(w.Stat, "router.go") {
		t.Errorf("summary does not name the file:\n%s", w.Stat)
	}
}

// TestDiffSummaryIsNeverTruncated is the reason stat is produced separately.
// A capped patch that also hid which files changed would leave the reader
// unable to tell what they had not seen.
func TestDiffSummaryIsNeverTruncated(t *testing.T) {
	dir, base := gittest.RepoWith(t, "seed.txt", "seed\n")

	// One file far past the budget, and a second small one after it
	// alphabetically, so a naive cap would lose the second entirely.
	big := strings.Repeat("a line of text that is long enough to add up\n", 4000)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "small.txt"), []byte("one line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := Diff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Truncated {
		t.Fatalf("patch of %d bytes was not truncated", len(w.Patch))
	}
	if len(w.Patch) > diffBudget+len(truncationNote) {
		t.Errorf("patch is %d bytes, over the %d budget", len(w.Patch), diffBudget)
	}
	for _, name := range []string{"big.txt", "small.txt"} {
		if !strings.Contains(w.Stat, name) {
			t.Errorf("summary lost %s, so the reader cannot tell what was cut:\n%s", name, w.Stat)
		}
	}
	if !strings.Contains(w.Patch, "truncated") {
		t.Error("a cut patch does not say it was cut")
	}
}

// TestDiffMeasuresFromTheMergeBase keeps a session's diff to its own work.
// Comparing against the parent branch's tip would attribute everything that
// landed there since the session started to this session as well.
func TestDiffMeasuresFromTheMergeBase(t *testing.T) {
	dir, base := gittest.RepoWith(t, "shared.txt", "one\n")

	// The session's own change, on a branch.
	run(dir, "checkout", "-q", "-b", "session/work")
	if err := os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(dir, "add", ".")
	run(dir, "commit", "-q", "-m", "session work")

	// Meanwhile main moves on with somebody else's commit.
	run(dir, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(dir, "theirs.txt"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(dir, "add", ".")
	run(dir, "commit", "-q", "-m", "someone else")
	run(dir, "checkout", "-q", "session/work")

	w, err := Diff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Stat, "mine.txt") {
		t.Errorf("the session's own work is missing:\n%s", w.Stat)
	}
	if strings.Contains(w.Stat, "theirs.txt") {
		t.Errorf("another branch's commit is credited to this session:\n%s", w.Stat)
	}
}

// TestDiffRefusesWithoutABase covers sessions recorded before BaseRef existed.
func TestDiffRefusesWithoutABase(t *testing.T) {
	dir, _ := gittest.RepoWith(t, "a.txt", "a\n")
	if _, err := Diff(dir, ""); err == nil {
		t.Error("a worktree with no recorded base produced a diff anyway")
	}
}

// TestDiffSeesNewFiles is the defect the truncation test exposed. A plain
// "git diff" ignores untracked files, so a session whose work is mostly new
// files read as having done nothing at all.
func TestDiffSeesNewFiles(t *testing.T) {
	dir, base := gittest.RepoWith(t, "seed.txt", "seed\n")
	if err := os.WriteFile(filepath.Join(dir, "handler.go"),
		[]byte("package api\n\nfunc Handle() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := Diff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Stat, "handler.go") {
		t.Errorf("a new file is missing from the summary:\n%s", w.Stat)
	}
	if !strings.Contains(w.Patch, "func Handle()") {
		t.Errorf("a new file's contents are missing from the patch:\n%s", w.Patch)
	}
}

// TestDiffLeavesTheWorktreeAlone pins the side-effect budget. Staging happens
// in a throwaway index, so the session's own index and working tree must be
// exactly as they were.
func TestDiffLeavesTheWorktreeAlone(t *testing.T) {
	dir, base := gittest.RepoWith(t, "seed.txt", "seed\n")
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := run(dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Diff(dir, base); err != nil {
		t.Fatal(err)
	}

	after, err := run(dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("Diff changed the session's own state:\nbefore %q\nafter  %q", before, after)
	}
	if !strings.Contains(after, "?? new.txt") {
		t.Errorf("the new file was staged in the session's index: %q", after)
	}
}

// TestDiffIgnoresIgnoredFiles keeps build output out of a review.
func TestDiffIgnoresIgnoredFiles(t *testing.T) {
	dir, base := gittest.RepoWith(t, ".gitignore", "build/\n")
	if err := os.MkdirAll(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build", "out.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real change beside the ignored one. Without it the assertion below
	// passes just as well when Diff returns nothing at all, which is the one
	// failure it most needs to catch.
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := Diff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Stat, "handler.go") {
		t.Fatalf("the tracked change is missing, so this proves nothing about ignoring:\n%s", w.Stat)
	}
	if strings.Contains(w.Stat, "out.bin") {
		t.Errorf("ignored build output appears in the review:\n%s", w.Stat)
	}
}

// TestDiffReportsAnUnrelatedHistory pins the failure rather than a plausible
// answer. Falling back to the base on a merge-base failure would return a Base
// field identical to the one a success produces, so the caller could not tell a
// real diff from a guess.
func TestDiffReportsAnUnrelatedHistory(t *testing.T) {
	dir, base := gittest.RepoWith(t, "seed.txt", "seed\n")

	// An orphan branch shares no commit with base, so there is no merge base.
	if _, err := run(dir, "checkout", "-q", "--orphan", "elsewhere"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(dir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := run(dir, "commit", "-q", "-m", "unrelated"); err != nil {
		t.Fatal(err)
	}

	if _, err := Diff(dir, base); err == nil {
		t.Error("an unrelated history produced a diff instead of an error")
	}
}
