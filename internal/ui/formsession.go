package ui

// The new-session form: its field order, and the choices each field offers.

import (
	"github.com/charmbracelet/bubbles/textinput"
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
	title := textinput.New()
	title.Placeholder = "what should this session do?"
	title.CharLimit = 120
	title.Prompt = ""

	f := &form{
		kind:  formNewSession,
		title: "New session",
		hint:  "tab/↑↓ field · ←/→ choose · ↵ next · ^s start · esc cancel",
		fields: []field{
			{kind: fieldChoice, label: "Project", selected: selected, choices: projects,
				pickable: true,
				help:     "←/→ to step, ↵ to choose from the full list"},
			{kind: fieldText, label: "Title", input: title,
				help: "Shown on the sidebar card. Not sent to the agent."},
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
