package gitx

// What git records about linked worktrees, read from `git worktree list
// --porcelain`, for the cases where the tree itself is no longer there to ask.

import (
	"path/filepath"
	"strings"
)

// worktreeRecord is git's record of one linked worktree.
type worktreeRecord struct {
	head     string
	detached bool
}

// missingRecord returns git's record of the worktree at dest, or nil when git
// has none.
//
// git prints each path resolved through symlinks, so dest is resolved the same
// way before comparing. It cannot be resolved directly when it is gone, so the
// deepest ancestor that still exists is resolved and the rest rejoined.
func missingRecord(repo, dest string) (*worktreeRecord, error) {
	out, err := run(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	want := resolveExisting(dest)
	for block := range strings.SplitSeq(out, "\n\n") {
		var rec worktreeRecord
		path := ""
		for line := range strings.SplitSeq(block, "\n") {
			key, val, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				path = val
			case "HEAD":
				rec.head = val
			case "detached":
				rec.detached = true
			}
		}
		if path == want {
			return &rec, nil
		}
	}
	return nil, nil
}

// resolveExisting resolves symlinks in the part of path that exists.
func resolveExisting(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	dir, rest := abs, ""
	for {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}
