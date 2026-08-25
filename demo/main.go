// Command demo seeds a throwaway workspace so Deck can be photographed
// against invented data.
//
// Screenshots for the README have to come from somewhere. Taking them against
// the real store would publish the name of every project on the machine, so
// this builds a self-contained one instead: three git repositories with a
// commit each, a state directory beside them, and sessions already recorded
// against the first repository.
//
//	go run ./demo                       # -> /tmp/deck-demo
//	go run ./demo -dir ~/deck-demo      # somewhere else
//
// It prints the command to run afterwards. Nothing here writes outside -dir.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// demoProject is one repository to create and register.
type demoProject struct {
	name, description string
	age               time.Duration // how long ago it was "added"
	file, content     string        // the one file its commit contains
}

// demoSession is one session recorded against the first project. Isolated
// sessions need a worktree that exists, so each of these gets a real one.
//
// The three name three different agents, so one screenshot of the sidebar
// shows that the agent is a per-session choice rather than a build option.
type demoSession struct {
	name, title, agent string
	age                time.Duration
}

// Ages are set back deliberately. A store built a moment ago reports "0s"
// against every row, which makes the app look like it has never been used.
var demoProjects = []demoProject{
	{"api-gateway", "request routing and auth", 11 * 24 * time.Hour,
		"router.go", "package gateway\n\n// Route matches a request to a backend.\nfunc Route(path string) string { return path }\n"},
	{"billing-service", "invoices and dunning", 4 * 24 * time.Hour,
		"invoice.go", "package billing\n\n// Total sums the lines of an invoice.\nfunc Total(lines []int) int { return 0 }\n"},
	{"web-frontend", "", 2 * time.Hour,
		"app.js", "export function mount(el) {\n  el.textContent = 'hello'\n}\n"},
}

var demoSessions = []demoSession{
	{"scheming-hawk-jhgk", "rate limit the public endpoints", "claude", 3 * time.Hour},
	{"wily-crane-bbbb", "split the auth middleware", "cathode", 95 * time.Minute},
	{"brisk-heron-cccc", "retry budget for upstream calls", "codex", 12 * time.Minute},
}

func main() {
	dir := flag.String("dir", "/tmp/deck-demo", "where to build the workspace")
	force := flag.String("force", "", "pass the -dir value again to overwrite an existing workspace")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		fail("resolve -dir: %v", err)
	}
	if err := prepare(root, *force); err != nil {
		fail("%v", err)
	}
	// Resolve symlinks before anything is recorded. On macOS /tmp is a link to
	// /private/tmp, so a project stored under one and launched from the other
	// does not match, and Deck registers a second copy of the same repository.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		fail("resolve %s: %v", root, err)
	}
	// store reads XDG_STATE_HOME on every call, so setting it here is what
	// keeps every path this writes inside root.
	stateHome := filepath.Join(root, "state")
	if err := os.Setenv("XDG_STATE_HOME", stateHome); err != nil {
		fail("set XDG_STATE_HOME: %v", err)
	}
	if err := seed(root); err != nil {
		fail("%v", err)
	}

	deck, err := filepath.Abs("./deck")
	if err != nil {
		fail("resolve the deck binary: %v", err)
	}
	// The cd matters. Deck registers its launch directory as a project, so
	// starting it from your own checkout puts that project's real name in the
	// screenshot. Starting inside a demo repository registers nothing new,
	// because that repository is already in the store.
	fmt.Printf("\nWorkspace ready. Photograph it with:\n\n")
	fmt.Printf("  cd %s \\\n", filepath.Join(root, "code", demoProjects[0].name))
	fmt.Printf("    && XDG_STATE_HOME=%s %s\n\n", stateHome, deck)
	fmt.Printf("Start inside a demo repo, not your own: Deck registers its launch\n")
	fmt.Printf("directory, and from elsewhere that adds a real project to the list.\n\n")
	fmt.Printf("Everything it writes stays under %s. Delete that to clean up.\n", root)
}

// prepare makes root, refusing to overwrite a directory that already holds a
// workspace unless the caller names it again. Screenshots get retaken, and a
// silent wipe of the wrong path is the one mistake that would cost real work.
func prepare(root, force string) error {
	if _, err := os.Stat(root); err == nil {
		if force == "" {
			return fmt.Errorf("%s exists — pass -force %s to replace it", root, root)
		}
		abs, err := filepath.Abs(force)
		if err != nil || abs != root {
			return fmt.Errorf("-force must repeat the -dir value exactly")
		}
		if err := os.RemoveAll(root); err != nil {
			return fmt.Errorf("replace %s: %w", root, err)
		}
	}
	return os.MkdirAll(filepath.Join(root, "code"), 0o755)
}

// seed creates the repositories, their worktrees and the state file.
func seed(root string) error {
	st := &store.State{}
	now := time.Now()
	var first *store.Project

	for _, p := range demoProjects {
		path := filepath.Join(root, "code", p.name)
		if err := initRepo(path, p.file, p.content); err != nil {
			return err
		}
		added := st.AddProject(store.Project{
			Name: p.name, Path: path, Description: p.description,
			CreatedAt: now.Add(-p.age),
		})
		if first == nil {
			first = added
		}
		fmt.Printf("  repo    %s\n", path)
	}

	worktrees, err := store.WorktreeDir()
	if err != nil {
		return err
	}
	for _, s := range demoSessions {
		dest := filepath.Join(worktrees, s.name)
		if err := gitx.AddWorktree(first.Path, dest, "session/"+s.name); err != nil {
			return fmt.Errorf("worktree for %s: %w", s.name, err)
		}
		st.AddSession(store.Session{
			ProjectID: first.ID, Name: s.name, Title: s.title, Agent: s.agent,
			Isolated: true, Branch: "session/" + s.name, Dir: dest,
			CreatedAt: now.Add(-s.age),
		})
		fmt.Printf("  session %s  (%s)\n", s.name, s.agent)
	}

	if err := st.Save(); err != nil {
		return err
	}
	return store.DefaultSettings().Save()
}

// initRepo creates a git repository holding one commit.
func initRepo(path, file, content string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, file), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	// The identity is set per repository rather than read from the machine, so
	// a screenshot of the log cannot carry the photographer's name.
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "deck demo"},
		{"config", "user.email", "demo@example.test"},
		{"add", "."},
		{"commit", "-q", "-m", "initial commit"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s in %s: %v: %s", args[0], path, err, out)
		}
	}
	return nil
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "demo: "+format+"\n", a...)
	os.Exit(1)
}
