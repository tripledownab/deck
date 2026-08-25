package ui

// The two modal forms — opening a session and registering a project — and how
// they are drawn. formfields.go holds the pieces, forminput.go the keys.

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type formKind int

const (
	formNewSession formKind = iota
	formAddProject
)

const (
	fieldText fieldKind = iota
	fieldChoice
)

// form is the modal used for the two flows that need input: opening a session
// and registering a project. It is deliberately small — two or three fields,
// no nesting, no validation framework. Anything larger belongs in a library.
type form struct {
	kind    formKind
	title   string
	hint    string
	fields  []field
	index   int
	problem string
}

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

// newProjectForm confirms a repository chosen in the directory explorer.
//
// The path arrives already validated as a git root, so the field is prefilled
// and focus starts on the description — the only thing left to type. It stays
// editable rather than being shown as static text, so a path can still be
// pasted or corrected without reopening the explorer.
func newProjectForm(repoPath string) *form {
	path := textinput.New()
	path.Placeholder = "/Users/you/Work/some-repo"
	path.CharLimit = 400
	path.Prompt = ""
	path.SetValue(repoPath)

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
			{kind: fieldText, label: "Description", input: desc, optional: true,
				help: "Optional."},
		},
	}
	f.focus(1)
	return f
}

func (f *form) view(s styleSet, width, height int) string {
	boxWidth := min(width-8, 76)
	if boxWidth < 30 {
		boxWidth = max(width-4, 20)
	}
	inner := boxWidth - 4

	var b strings.Builder
	b.WriteString(s.Title.Render(f.title))
	b.WriteString("\n")
	b.WriteString(s.Rule.Render(strings.Repeat("─", inner)))
	b.WriteString("\n\n")

	for i := range f.fields {
		fl := &f.fields[i]
		marker := "  "
		labelStyle := s.Label
		if i == f.index {
			marker = s.Accent.Render("▸ ")
			labelStyle = s.Value.Bold(true)
		}
		b.WriteString(marker + labelStyle.Render(fl.label) + "\n")

		switch fl.kind {
		case fieldText:
			fl.input.Width = inner - 4
			b.WriteString("  " + fl.input.View() + "\n")
		case fieldChoice:
			b.WriteString(renderChoices(s, fl, inner) + "\n")
		}

		help := fl.help
		if fl.kind == fieldChoice {
			help = fl.choices[fl.selected].help
		}
		if help != "" {
			b.WriteString("  " + s.Faint.Render(wrap(help, inner-2)) + "\n")
		}
		b.WriteString("\n")
	}

	if f.problem != "" {
		// Wrap: a git failure is a sentence, not a label, and an unwrapped one
		// runs past the box.
		b.WriteString(s.Error.Render("! "+wrap(f.problem, inner-2)) + "\n\n")
	}
	b.WriteString(s.Footer.Render(f.hint))

	box := s.Modal.Width(boxWidth).Padding(1, 2).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
