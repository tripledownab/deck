package ui

// The project forms: registering a directory, and renaming one that is
// already registered.

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/tripledownab/deck/internal/store"
)

// Field indices in the add-project form. Named for the same reason the
// session form's are: commitForm reads them positionally, and inserting a
// field without these would silently send the name where the description goes.
const (
	projectFieldPath = iota
	projectFieldName
	projectFieldDescription
)

// newProjectForm confirms a repository chosen in the directory explorer.
//
// The path arrives already validated as a git root, so the field is prefilled
// and focus starts on the name — the first thing left to type. It stays
// editable rather than being shown as static text, so a path can still be
// pasted or corrected without reopening the explorer.
func newProjectForm(repoPath string) *form {
	path := textinput.New()
	path.Placeholder = "/Users/you/Work/some-repo"
	path.CharLimit = 400
	path.Prompt = ""
	path.SetValue(repoPath)

	// The placeholder is the directory name, which is what an empty field
	// falls back to. It is a hint rather than the decision: addProject derives
	// the fallback from the resolved root, so editing the path above still
	// gets the right default even once this line is stale.
	name := textinput.New()
	name.Placeholder = filepath.Base(repoPath)
	name.CharLimit = 80
	name.Prompt = ""

	desc := textinput.New()
	desc.Placeholder = "one line, shown on the dashboard"
	desc.CharLimit = 160
	desc.Prompt = ""

	f := &form{
		kind:  formAddProject,
		title: "Add project",
		hint:  "tab/↑↓ move · ^s add · esc cancel",
		fields: []field{
			{kind: fieldText, label: "Repository path", input: path,
				help: "Must be inside a git working tree."},
			{kind: fieldText, label: "Name", input: name, optional: true,
				help: "Shown in the sidebar. Defaults to the directory name."},
			{kind: fieldText, label: "Description", input: desc, optional: true,
				help: "Optional."},
		},
	}
	f.focus(projectFieldName)
	return f
}

// Field indices in the rename form. Its own set rather than the add form's:
// there is no path field here, so the positions differ, and sharing constants
// between two shapes is how the wrong string reaches the store.
const (
	editFieldName = iota
	editFieldDescription
)

// editProjectForm renames a project that is already registered.
//
// The path is not a field. Changing it would make this a different project
// rather than the same one under another name, and the registration flow
// already exists for that. It is shown in the hint line as context instead.
func editProjectForm(p *store.Project) *form {
	name := textinput.New()
	name.Placeholder = filepath.Base(p.Path)
	name.CharLimit = 80
	name.Prompt = ""
	name.SetValue(p.Name)

	desc := textinput.New()
	desc.Placeholder = "one line, shown on the dashboard"
	desc.CharLimit = 160
	desc.Prompt = ""
	desc.SetValue(p.Description)

	f := &form{
		kind:    formEditProject,
		title:   "Rename project",
		hint:    "tab/↑↓ move · ^s save · esc cancel",
		subject: p.ID,
		fields: []field{
			{kind: fieldText, label: "Name", input: name, optional: true,
				help: "Shown in the sidebar. Defaults to the directory name."},
			{kind: fieldText, label: "Description", input: desc, optional: true,
				help: "Optional."},
		},
	}
	f.focus(editFieldName)
	return f
}
