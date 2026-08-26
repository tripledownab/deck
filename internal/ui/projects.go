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

// openEditProjectForm renames the project under the cursor.
func (m Model) openEditProjectForm() (tea.Model, tea.Cmd) {
	p := m.currentProject()
	if p == nil {
		m.notice = "no project to rename"
		return m, nil
	}
	m.form = editProjectForm(p)
	return m, nil
}

// renameProject applies the rename form.
//
// The project is found by id, not by cursor position: the form may have been
// open while the selection moved, and writing to whatever is selected now
// would rename the wrong row.
func (m Model) renameProject(id, name, description string) (tea.Model, tea.Cmd) {
	p := m.state.Project(id)
	if p == nil {
		m.formProblem(fmt.Errorf("that project is no longer registered"))
		return m, nil
	}
	// Same fallback as registering: an empty name is the directory it sits in,
	// so a cleared field cannot leave a blank row in the sidebar.
	if name == "" {
		name = filepath.Base(p.Path)
	}
	p.Name, p.Description = name, description
	if err := m.state.Save(); err != nil {
		m.formProblem(err)
		return m, nil
	}
	m.form = nil
	m.notice = "renamed to " + p.Name
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

// addProject registers a directory. An empty name falls back to the directory
// it sits in, so the sidebar always has something to show; the fallback is
// derived from the resolved root rather than from what was typed, because
// resolveProject may have climbed to a repository root above it.
func (m Model) addProject(path, name, description string) (tea.Model, tea.Cmd) {
	root, err := resolveProject(path)
	if err != nil {
		m.formProblem(err)
		return m, nil
	}
	if existing := m.state.ProjectByPath(root); existing != nil {
		m.formProblem(fmt.Errorf("%s is already registered as %q", root, existing.Name))
		return m, nil
	}
	if name == "" {
		name = filepath.Base(root)
	}
	p := m.state.AddProject(store.Project{
		Name:        name,
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
