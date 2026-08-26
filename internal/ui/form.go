package ui

// The modal form widget and how it is drawn. The forms themselves are built
// in formsession.go and formproject.go; formfields.go holds the pieces and
// forminput.go the keys.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type formKind int

const (
	formNewSession formKind = iota
	formAddProject
	formEditProject
)

const (
	fieldText fieldKind = iota
	fieldChoice
)

// form is the modal used for the flows that need input: opening a session,
// registering a project, and renaming one. It is deliberately small — two to
// four fields, no nesting, no validation framework. Anything larger belongs in
// a library.
//
// subject carries the id the form acts on when it edits something that already
// exists. Editing by id rather than by list position means a form left open
// while the cursor moves still writes to the row it was opened on.
type form struct {
	kind    formKind
	title   string
	hint    string
	subject string
	fields  []field
	index   int
	problem string
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
