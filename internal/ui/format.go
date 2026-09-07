package ui

// Turning model values into the short strings a row can hold.

import (
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
