package ui

// Turning model values into the short strings a row can hold: what a project
// and a session are called.

import (
	"path/filepath"

	"github.com/tripledownab/deck/internal/store"
)

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
