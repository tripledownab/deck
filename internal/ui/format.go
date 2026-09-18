package ui

// Turning model values into the short strings a row can hold: what a project
// and a session are called, and how a figure is abbreviated to fit beside them.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// compactCount renders a token count for a sidebar badge.
//
// Thousands are abbreviated because the badge shares a status line that
// truncates from the right, and the digits past the first two are noise on a
// figure that changes several times a second. 950 stays 950; 1500 becomes
// 1.5k; 23800 becomes 24k, since a tenth of a thousand is below what anyone
// reads at that size.
func compactCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 10000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
}

// sessionLabel is what a session is called on screen.
//
// The title if it has one, otherwise the generated name. A session is created
// with a title and Deck never clears it, so the fallback is for a session
// recorded before titles existed and for one titled with whitespace.
func sessionLabel(sess store.Session) string {
	if sess.Title != "" {
		return sess.Title
	}
	return sess.Name
}

// projectLabel is what a project is called on screen.
//
// Name is filled in from the directory when the field is left empty, both when
// registering and when renaming, so the fallback here is not the usual path. It
// stands for a hand-edited state file: a blank row in a list of projects tells
// the user nothing about which one it is.
func projectLabel(state *store.State, projectID string) string {
	p := state.Project(projectID)
	if p == nil {
		return "unknown project"
	}
	if p.Name != "" {
		return p.Name
	}
	return filepath.Base(p.Path)
}

// workSummary is the one line a modal row can hold about what a session
// changed: "3 files changed, 41 insertions(+)", or "no files changed".
//
// gitx.Work.Stat is one line per changed file followed by that total, and the
// total is the last of them. Diff substitutes "no files changed" for an empty
// stat, so there is always a line to take.
//
// A failed measurement is reported in place of the figure. A row that said "no
// files changed" because git errored would be a confirmation stating a fact it
// does not have, which is worse than one admitting it could not look.
func workSummary(sess store.Session) string {
	w, err := gitx.Diff(sess.Dir, sess.BaseRef)
	if err != nil {
		return "could not measure what it changed: " + err.Error()
	}
	lines := strings.Split(strings.TrimSpace(w.Stat), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
