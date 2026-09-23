package ui

// The session forms: opening one, and retitling one that is already running.

import (
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/tripledownab/deck/internal/store"
)

// Field indices in the new-session form. Named because commitForm reads them
// positionally and a silent reorder would send the wrong string to git.
const (
	sessionFieldProject = iota
	sessionFieldTitle
	sessionFieldWorkingCopy
	sessionFieldAgent
)

// workingCopy indices, matching the choices built below.
const (
	workingCopyWorktree = 0
	workingCopyDir      = 1
)

// newSessionForm builds the new-session modal.
//
// The project is a field rather than context because the modal is reachable
// from a running session (^g n), where the only project on screen is the one
// you are already in. Without it you had to leave the session, go to the
// dashboard, and select another project first.
//
// canWorktree says whether the preselected project is a git repository. When
// it is not — a directory that only collects repositories, say — the default
// working copy is the project directory, because there is no branch to work
// from. Both choices stay on offer, since the project field can be changed
// without rebuilding the form; picking an impossible combination is caught on
// commit with a message that says which.
func newSessionForm(projects []choice, selected int, canWorktree bool, agent string) *form {
	f := &form{
		kind:  formNewSession,
		title: "New session",
		hint:  "tab/↑↓ field · ←/→ choose · ↵ next · ^s start · esc cancel",
		fields: []field{
			{kind: fieldChoice, label: "Project", selected: selected, choices: projects,
				pickable: true,
				help:     "←/→ to step, ↵ to choose from the full list"},
			titleField(""),
			{kind: fieldChoice, label: "Working copy", selected: defaultWorkingCopy(canWorktree), choices: []choice{
				{label: "Isolated git worktree", value: "worktree",
					help: "New branch session/<name> checked out under the state dir. Parallel sessions never collide."},
				{label: "Project directory", value: "cwd",
					help: "Runs in the repo itself. Simple, but two sessions here fight over the working tree."},
			}},
			agentField(agent),
		},
	}
	// Start on the title: the project is usually already right, and typing is
	// the only thing that always has to happen.
	f.focus(sessionFieldTitle)
	return f
}

// titleField builds the Title input both session forms carry.
//
// One builder rather than a copy in each: the two forms write the same value
// to the same place, and a placeholder or a limit raised in one of them would
// make the rename form describe a different field from the one that created
// the session.
func titleField(value string) field {
	title := textinput.New()
	title.Placeholder = "what should this session do?"
	title.CharLimit = 120
	title.Prompt = ""
	title.SetValue(value)
	return field{kind: fieldText, label: "Title", input: title,
		help: "Shown on the sidebar card. Not sent to the agent."}
}

// The rename form's only field. Named for the same reason the others are:
// commitForm reads it positionally.
const editSessionFieldTitle = 0

// editSessionForm renames a session that already exists.
//
// The title is the only field, because it is the only name a session has that
// nothing else is built on. Name and Branch are in the worktree path and in
// git, so editing them here would rename neither and leave both pointing at a
// session that no longer claims them.
//
// The generated name is in the heading rather than in a field: it says which
// row this form was opened on, which the title alone cannot once it is being
// replaced.
func editSessionForm(sess *store.Session) *form {
	f := &form{
		kind:    formEditSession,
		title:   "Rename " + sess.Name,
		hint:    "↵ or ^s save · esc cancel",
		subject: sess.ID,
		fields:  []field{titleField(sess.Title)},
	}
	f.focus(editSessionFieldTitle)
	return f
}
