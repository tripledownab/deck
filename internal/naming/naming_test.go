package naming

import (
	"regexp"
	"strings"
	"testing"
)

// The suffix alphabet drops l and o (and 0 and 1) because they are misread
// when a name is spoken aloud; every other lowercase letter and 2-9 is in.
var sessionPattern = regexp.MustCompile(`^[a-z]+-[a-z]+-[a-kmnp-z2-9]{4}$`)

func TestSessionShape(t *testing.T) {
	for range 200 {
		got := Session()
		if !sessionPattern.MatchString(got) {
			t.Fatalf("Session() = %q, does not match %s", got, sessionPattern)
		}
	}
}

// TestSessionIsUnique guards the reason the suffix exists. Two sessions on one
// project must not collide, or the second `git worktree add` fails.
func TestSessionIsUnique(t *testing.T) {
	const draws = 2000
	seen := make(map[string]bool, draws)
	collisions := 0
	for range draws {
		n := Session()
		if seen[n] {
			collisions++
		}
		seen[n] = true
	}
	// 50 adjectives x 50 animals x 32^4 suffixes is a large space; a couple of
	// birthday collisions in 2000 draws would still be suspicious.
	if collisions > 0 {
		t.Errorf("%d collisions in %d draws", collisions, draws)
	}
}

func TestBranch(t *testing.T) {
	if got := Branch("scheming-hawk-jhgk"); got != "session/scheming-hawk-jhgk" {
		t.Errorf("Branch() = %q", got)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Two Words":        "two-words",
		"  leading space":  "leading-space",
		"UPPER_snake.case": "upper-snake-case",
		"already-slugged":  "already-slugged",
		"!!!":              "",
		"a  b":             "a-b",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSlugIsPathSafe matters because the slug becomes a directory name under
// the state dir.
func TestSlugIsPathSafe(t *testing.T) {
	got := Slug("../../etc/passwd")
	if strings.Contains(got, "/") || strings.Contains(got, "..") {
		t.Errorf("Slug produced a traversable path: %q", got)
	}
}
