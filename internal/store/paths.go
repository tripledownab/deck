package store

// Where Deck keeps things on disk. Every path goes through here so no
// caller re-derives one.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// NewID returns a short random identifier. Sixteen hex characters is far more
// than we need to avoid a collision and still fits a debug log line.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is not a condition we can paper over: every
		// identifier in the store would collide on the zero value.
		panic("deck: cannot read random bytes: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// Dir returns Deck's state directory, honouring XDG_STATE_HOME. This is
// the same convention cathode uses, so both tools sit side by side under the
// user's state root.
func Dir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "deck"), nil
}

// WorktreeDir returns the root under which Deck creates session worktrees.
// They live in the state directory rather than beside the repo so that a
// project checkout stays clean and `git status` never reports them.
func WorktreeDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "worktrees"), nil
}

// NotesDir is where the shared per-project agent logs live. Outside every
// worktree on purpose: a worktree is a separate checkout, so anything written
// inside one is invisible to its siblings.
func NotesDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "notes"), nil
}

// SessionConfigDir is where per-session files that are not the worktree live —
// currently the generated MCP config and status-hooks settings handed to the
// agent.
func SessionConfigDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sessions"), nil
}

func statePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}
