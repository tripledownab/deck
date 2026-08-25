package ui

// The help modal, rendered from the key table so a rebind cannot drift from
// its own documentation.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) helpView() string {
	s := m.styles

	var b strings.Builder
	b.WriteString(s.Title.Render("Deck keys") + "\n\n")

	for _, g := range m.keys.helpGroups() {
		b.WriteString(s.GroupLabel.Render(g.title) + "\n")
		for _, k := range g.keys {
			h := k.Help()
			b.WriteString("  " + s.Key.Render(pad(h.Key, 12)) + s.Muted.Render(h.Desc) + "\n")
		}
		b.WriteString("\n")
	}

	// Two rows the bindings cannot express: a literal prefix is not a binding,
	// and the last line is a fact about dispatch rather than a key.
	b.WriteString("  " + s.Key.Render(pad("^g ^g", 12)) +
		s.Muted.Render("send a literal "+PrefixKey+" to the agent") + "\n\n")
	b.WriteString(s.Muted.Render("While attached, every other key goes to the agent.") + "\n")
	b.WriteString(s.Muted.Render("Press ^g on its own to see the command list in the footer.") + "\n")
	b.WriteString("\n" + s.Footer.Render("any key to close"))

	box := s.Modal.Padding(1, 3).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
