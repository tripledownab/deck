// Command deck is Deck: a terminal workspace for parallel coding-agent
// sessions.
//
// It manages projects and sessions and draws the frame around them. The agent
// itself is a separate program running in a pseudo-terminal — `claude` by
// default, or `cathode`, or anything else that talks to a terminal. Deck
// speaks no agent protocol, which is why the agent is a per-session choice.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
	"github.com/tripledownab/deck/internal/ui"
)

func main() {
	agentCmd := flag.String("agent", "", "agent executable for this run, overriding the remembered choice (e.g. claude, cathode)")
	agentArgs := flag.String("agent-args", "", "extra arguments passed to the agent, space separated")
	flag.Parse()

	state, err := store.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		fmt.Fprintln(os.Stderr, "the state file exists but could not be read; fix or remove it rather than losing track of your sessions")
		os.Exit(1)
	}

	here, err := registerCwd(state)
	if err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		os.Exit(1)
	}

	settings := store.LoadSettings()
	m := ui.New(state, *agentCmd, strings.Fields(*agentArgs)).
		WithSettings(settings).
		FocusProject(here)

	// The coordination server lets agents on one project see each other's
	// claims and notes. A failure here is reported, not fatal: sessions still
	// run, they are just blind to each other, and the user should know which
	// of those two worlds they are in.
	if notes, err := store.NotesDir(); err != nil {
		fmt.Fprintln(os.Stderr, "deck: agent coordination disabled:", err)
	} else if c, err := coord.Start(notes); err != nil {
		fmt.Fprintln(os.Stderr, "deck: agent coordination disabled:", err)
	} else {
		defer c.Close()
		m = m.WithCoordinator(c)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	final, err := p.Run()

	// Stop the agents here, after the program has exited. Doing it from the
	// update loop would block the loop that keeps their output draining.
	if fm, ok := final.(ui.Model); ok {
		fm.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		os.Exit(1)
	}
}

// registerCwd adds the repository we were launched in to the store, unless it
// is already there, and returns its root.
//
// The store is global, not per-directory: every project and every session is
// visible from wherever deck runs, and a session's agent runs in that
// session's own directory regardless of the launch directory. So registering
// the cwd is not about scoping — it is about the one project you are most
// likely to want, which is the one you are standing in.
//
// This used to fire only when the store was empty, which meant cd-ing into a
// new repository and running deck showed every project except that one.
func registerCwd(state *store.State) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	root, err := gitx.RepoRoot(cwd)
	if err != nil {
		// Not a repository itself. Register it anyway if it directly holds
		// repositories — a collector coordinating several checkouts is a
		// project, and `deck` inside one should find it already there.
		//
		// Anything else is left alone: a project is added deliberately with
		// `a`, and seeding every directory anyone ever ran deck in would fill
		// the dashboard with Downloads folders.
		if !gitx.HoldsRepos(cwd) {
			return "", nil
		}
		root = cwd
		if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
			root = resolved
		}
	}
	if state.ProjectByPath(root) != nil {
		return root, nil
	}
	state.AddProject(store.Project{Name: filepath.Base(root), Path: root})
	return root, state.Save()
}
