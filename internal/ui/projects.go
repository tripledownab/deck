package ui

// Registering a project: turning a typed or browsed path into the path to
// store, and the explorer that finds one.

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// openBrowser starts the directory explorer for adding a project.
func (m Model) openBrowser() (tea.Model, tea.Cmd) {
	start := ""
	if p := m.currentProject(); p != nil {
		start = p.Path
	}
	m.browser = newBrowser(browserStart(start), clamp(m.height-16, 5, 16))
	return m, nil
}

// resolveProject turns a user-supplied path into the path to store.
//
// A path inside a git repository collapses to that repository's root, so one
// project cannot be registered twice through two of its own subdirectories.
// Anything else is kept as it is: **a project does not have to be a
// repository.** A directory that merely collects them — several checkouts side
// by side, coordinated from the parent — is a project in its own right, and an
// agent running there can see all of them at once.
//
// Symlinks are resolved because git reports resolved paths, and two spellings
// of one directory would otherwise register as two projects.
func resolveProject(path string) (string, error) {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file, not a directory", abs)
	}
	if root, err := gitx.RepoRoot(abs); err == nil {
		return root, nil
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return filepath.Clean(abs), nil
}

func (m Model) addProject(path, description string) (tea.Model, tea.Cmd) {
	root, err := resolveProject(path)
	if err != nil {
		m.formProblem(err)
		return m, nil
	}
	if existing := m.state.ProjectByPath(root); existing != nil {
		m.formProblem(fmt.Errorf("%s is already registered as %q", root, existing.Name))
		return m, nil
	}
	p := m.state.AddProject(store.Project{
		Name:        filepath.Base(root),
		Path:        root,
		Description: description,
	})
	if err := m.state.Save(); err != nil {
		m.formProblem(err)
		return m, nil
	}
	m.form = nil
	m.projectIx = len(m.state.Projects) - 1
	m.notice = "added " + p.Name
	return m, nil
}

// expandHome resolves a leading ~ so a typed path behaves like it would in a
// shell.
func expandHome(path string) string {
	if path == "~" || len(path) > 1 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
