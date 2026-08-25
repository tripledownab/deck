// Package naming generates the human-readable session names Deck puts on
// branches, worktree directories, and sidebar cards — "scheming-hawk-jhgk".
//
// The trailing suffix is what makes the name unique; the adjective and animal
// exist so a person can say which session they mean out loud. Never drop the
// suffix to make a name prettier: two sessions on one project would then
// collide on the branch name and the second `git worktree add` would fail.
package naming

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode"
)

var adjectives = []string{
	"amber", "arid", "brave", "brisk", "chaotic", "civil", "clever", "coastal",
	"cosmic", "curious", "dapper", "dawn", "eager", "electric", "fabled",
	"feral", "fluent", "frosty", "gallant", "gentle", "gilded", "glacial",
	"hidden", "humble", "idle", "jolly", "keen", "lucid", "lunar", "mellow",
	"nimble", "noble", "polar", "prime", "quiet", "rapid", "restless", "rogue",
	"scheming", "silent", "solar", "stoic", "sudden", "swift", "tidal",
	"upbeat", "vivid", "wandering", "wily", "zealous",
}

var animals = []string{
	"albatross", "badger", "beetle", "bison", "cobra", "condor", "coyote",
	"crane", "dingo", "falcon", "ferret", "gecko", "gibbon", "hawk", "heron",
	"ibex", "jackal", "kestrel", "lemur", "lynx", "macaw", "magpie", "marlin",
	"mongoose", "narwhal", "ocelot", "orca", "osprey", "otter", "panther",
	"pelican", "puffin", "quokka", "raven", "robin", "salmon", "seal", "shrike",
	"stoat", "tapir", "teal", "tern", "vulture", "walrus", "weasel", "wombat",
	"wren", "yak", "zebra", "zorilla",
}

const suffixAlphabet = "abcdefghijkmnpqrstuvwxyz23456789" // no l/o/0/1 — misread aloud

// Session returns a fresh name like "scheming-hawk-jhgk".
func Session() string {
	return fmt.Sprintf("%s-%s-%s", pick(adjectives), pick(animals), suffix(4))
}

// Branch returns the git branch name for a session name.
func Branch(sessionName string) string { return "session/" + sessionName }

func pick(from []string) string {
	return from[randIndex(len(from))]
}

func suffix(n int) string {
	var b strings.Builder
	for range n {
		b.WriteByte(suffixAlphabet[randIndex(len(suffixAlphabet))])
	}
	return b.String()
}

// randIndex returns a uniform index below n. It draws from crypto/rand and
// rejects the tail of the byte range that would bias short alphabets.
func randIndex(n int) int {
	if n <= 0 {
		panic("naming: empty word list")
	}
	limit := 256 - (256 % n)
	var b [1]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			panic("naming: cannot read random bytes: " + err.Error())
		}
		if int(b[0]) < limit {
			return int(b[0]) % n
		}
	}
}

// Slug turns free text into a name safe for a branch or a directory. It is
// used for project directory names, never for session uniqueness.
func Slug(s string) string {
	var b strings.Builder
	lastDash := true // leading dashes are dropped
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
