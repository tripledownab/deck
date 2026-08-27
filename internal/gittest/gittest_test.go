package gittest

import (
	"path/filepath"
	"testing"
)

// TestRepoReturnsAResolvedPath pins the one property that made this package
// worth extracting. Four callers had grown their own fixture and they had
// already drifted on exactly this: t.TempDir hands back a symlinked path on
// macOS, git reports the resolved one, and a caller comparing the two sees a
// mismatch that has nothing to do with what it is testing. The same difference
// registered one directory as two projects elsewhere in this repository.
//
// Nothing else fails if the resolution is dropped, so without this the guard
// is a comment rather than a rule.
func TestRepoReturnsAResolvedPath(t *testing.T) {
	dir := Repo(t)

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dir != resolved {
		t.Errorf("Repo returned %q, which resolves to %q", dir, resolved)
	}

	// And git agrees, which is the comparison a caller actually makes.
	if got := Run(t, dir, "rev-parse", "--show-toplevel"); got != dir {
		t.Errorf("git reports %q, Repo returned %q", got, dir)
	}
}
