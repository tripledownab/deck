package gitx

// What a session has changed: the summary of a worktree against the commit it
// branched from, and the patch that goes with it.

import (
	"fmt"
	"os"
	"strings"
)

// diffBudget caps the patch a Diff returns.
//
// The reader is an agent's context window, not a terminal. 64 KiB is roughly
// sixteen thousand tokens, which is a large but survivable share of a turn,
// and well past the size at which a reviewer would still be reading closely.
// The stat summary is never counted against it — see Diff.
const diffBudget = 64 << 10

// truncationNote replaces the tail of a patch that ran past diffBudget. It
// names the budget so the reader can tell a short diff from a clipped one,
// and derives it rather than repeating it: a hard-coded size goes on telling
// the reader the old number the moment diffBudget changes.
var truncationNote = fmt.Sprintf(
	"\n… patch truncated at %d KiB. The summary above lists every changed file.\n",
	diffBudget>>10)

// Work is what one worktree has changed since it branched.
type Work struct {
	Base      string // the commit or ref the work is measured against
	Stat      string // one line per changed file, always complete
	Patch     string // the changes themselves, capped at diffBudget
	Truncated bool   // whether Patch was cut
}

// Diff reports the work in dir relative to base.
//
// It measures against the **merge base**, not against base itself. Comparing
// with base directly attributes everything that landed on the parent branch
// since the session started to this session, which is the opposite of what a
// reviewer is being asked to look at.
//
// Committed and uncommitted changes are both included: a session that has not
// committed yet has still done work, and a reviewer told "no changes" because
// nothing is committed has been misled.
//
// The stat summary is produced separately and is never truncated. A capped
// patch that also hid which files changed would leave the reader unable to
// tell what they had not seen.
func Diff(dir, base string) (Work, error) {
	if base == "" {
		return Work{}, fmt.Errorf("no base recorded for this worktree")
	}
	if _, err := RepoRoot(dir); err != nil {
		return Work{}, err
	}
	// Reported, not absorbed. Falling back to base on failure would return a
	// diff indistinguishable from a correct one — the Base field would read
	// exactly as it does when merge-base succeeds — and a repository where
	// merge-base fails is not one whose diff should be believed.
	point, err := run(dir, "merge-base", base, "HEAD")
	if err != nil {
		return Work{}, fmt.Errorf("no common history between %s and HEAD: %w", base, err)
	}

	stat, patch, err := diffIncludingNewFiles(dir, point)
	if err != nil {
		return Work{}, err
	}

	w := Work{Base: point, Stat: stat, Patch: patch}
	if len(w.Patch) > diffBudget {
		// Cut on a line boundary: half a hunk header is worse than one line
		// less of context.
		cut := strings.LastIndex(w.Patch[:diffBudget], "\n")
		if cut < 0 {
			cut = diffBudget
		}
		w.Patch = w.Patch[:cut] + truncationNote
		w.Truncated = true
	}
	if w.Stat == "" {
		w.Stat = "no files changed"
	}
	return w, nil
}

// diffIncludingNewFiles produces the summary and patch, counting files the
// session has created but not yet committed.
//
// A plain "git diff" ignores untracked files entirely, so a session whose work
// is mostly new files would have read as having done nothing — which is worse
// than an incomplete answer, because it looks like a definite one.
//
// Staging into a throwaway index is what makes them visible without touching
// anything the session owns. GIT_INDEX_FILE redirects the add, so the index
// the agent is using is untouched, as is its working tree. The only trace is
// the blobs written into .git/objects, which are unreferenced, inert, and the
// same objects a later "git add" would write anyway.
func diffIncludingNewFiles(dir, point string) (stat, patch string, err error) {
	idx, err := os.CreateTemp("", "deck-index-*")
	if err != nil {
		return "", "", fmt.Errorf("create temporary index: %w", err)
	}
	name := idx.Name()
	idx.Close()
	// git wants to create the file itself; an existing empty one is not a
	// valid index and it refuses to read it.
	os.Remove(name)
	defer os.Remove(name)

	env := []string{"GIT_INDEX_FILE=" + name}
	if _, err := runEnv(dir, env, "read-tree", "HEAD"); err != nil {
		return "", "", err
	}
	// -A stages deletions too, and still honours .gitignore, so build output
	// does not turn up in a review.
	if _, err := runEnv(dir, env, "add", "-A"); err != nil {
		return "", "", err
	}
	if stat, err = runEnv(dir, env, "diff", "--cached", "--stat", point); err != nil {
		return "", "", err
	}
	if patch, err = runEnv(dir, env, "diff", "--cached", point); err != nil {
		return "", "", err
	}
	return stat, patch, nil
}
