package ui

// The pieces a form is built from: a field, its choices, and how a choice row
// is drawn. form.go composes these into the two modals.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type fieldKind int

type choice struct {
	label string
	help  string
	value string
}

type field struct {
	kind     fieldKind
	label    string
	help     string
	input    textinput.Model
	choices  []choice
	selected int
	optional bool

	// pickable fields open the list picker on Enter instead of cycling.
	// Cycling is fine for two options and useless for fifty: the project list
	// grows with the user's machine, and ←/→ through it is not navigation.
	pickable bool
}

func (f *field) value() string {
	if f.kind == fieldChoice {
		return f.choices[f.selected].value
	}
	return strings.TrimSpace(f.input.Value())
}

// agentChoices are the programs the form offers.
//
// Deck speaks no agent protocol, so this list is a menu and not a
// capability boundary: any program that talks to a terminal runs in a pane,
// and -agent takes one that is not here. What differs between them is how much
// Deck can say about them — only claude reports turn status through hooks,
// and only claude and cathode take a coordination config, so the rest show the
// activity heuristic and cannot see their siblings.
var agentChoices = []choice{
	{label: "claude", value: "claude",
		help: "Claude Code. Exact turn status, and sees sibling sessions."},
	{label: "cathode", value: "cathode",
		help: "Single-session harness: inline diffs, approvals pane, sees siblings."},
	{label: "codex", value: "codex",
		help: "Runs in the pane. No coordination tools, status from pane activity."},
	{label: "gemini", value: "gemini",
		help: "Runs in the pane. No coordination tools, status from pane activity."},
}

// agentField builds the Agent choice, preselected on the one in use.
//
// An agent that is not one of the two known ones came from -agent, and is
// offered first: a form that cannot express the state it opened in would
// silently change the agent the moment you submitted it.
func agentField(agent string) field {
	f := field{kind: fieldChoice, label: "Agent",
		help: "Remembered as the default for the next session."}
	for i, c := range agentChoices {
		if c.value == agent {
			f.choices, f.selected = agentChoices, i
			return f
		}
	}
	f.choices = append([]choice{{label: agent, value: agent, help: "From -agent."}}, agentChoices...)
	return f
}

func defaultWorkingCopy(canWorktree bool) int {
	if canWorktree {
		return workingCopyWorktree
	}
	return workingCopyDir
}

// renderChoices lays a choice field out as a row of chips, falling back to a
// single cycler when they will not fit.
//
// Two options fit anywhere; a project list does not. Chips that overflow wrap
// into the field below and push the whole modal out of shape, so past the
// width budget the field shows only the selection and its position.
func renderChoices(s styleSet, fl *field, width int) string {
	chips := make([]string, 0, len(fl.choices))
	for j, c := range fl.choices {
		st := s.Chip
		if j == fl.selected {
			st = s.ChipActive
		}
		chips = append(chips, st.Render(c.label))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, chips...)
	if lipgloss.Width(row) <= width {
		return row
	}

	// Indented with a style, not a string prefix: the chip is three lines of
	// border and a "  " prefix only shifts the first of them.
	cur := fl.choices[fl.selected]
	chip := s.ChipActive.Render("‹ " + truncate(cur.label, width-14) + " ›")
	count := s.Faint.Render(fmt.Sprintf(" %d/%d", fl.selected+1, len(fl.choices)))
	return lipgloss.NewStyle().PaddingLeft(2).Render(
		lipgloss.JoinHorizontal(lipgloss.Bottom, chip, count))
}

// wrap breaks text at width on word boundaries. lipgloss can do this via
// Style.Width, but the help lines are plain text and this keeps them out of
// the style cache.
func wrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	var line string
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n  ")
}
