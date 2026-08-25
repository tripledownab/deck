package ui

// Every lipgloss style the app draws with, rebuilt from the active palette.

import "github.com/charmbracelet/lipgloss"

// styleSet is the built set of lipgloss styles for one palette.
type styleSet struct {
	P palette

	Wordmark   lipgloss.Style
	Tab        lipgloss.Style
	TabActive  lipgloss.Style
	Chip       lipgloss.Style
	ChipActive lipgloss.Style

	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted    lipgloss.Style
	Faint    lipgloss.Style
	Accent   lipgloss.Style
	Label    lipgloss.Style
	Value    lipgloss.Style
	Error    lipgloss.Style

	Rule       lipgloss.Style
	Pane       lipgloss.Style
	PaneActive lipgloss.Style
	Modal      lipgloss.Style
	Card       lipgloss.Style
	CardActive lipgloss.Style
	GroupLabel lipgloss.Style

	NavItem       lipgloss.Style
	NavItemActive lipgloss.Style
	Footer        lipgloss.Style
	Key           lipgloss.Style
}

// newStyles builds every style from a palette.
func newStyles(p palette) styleSet {
	s := styleSet{P: p}

	s.Wordmark = lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	s.Tab = lipgloss.NewStyle().Foreground(p.Muted).Padding(0, 1)
	s.TabActive = lipgloss.NewStyle().Foreground(p.Fg).Bold(true).Padding(0, 1).
		Underline(true)
	s.Chip = lipgloss.NewStyle().Foreground(p.Muted).
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).Padding(0, 1)
	s.ChipActive = lipgloss.NewStyle().Foreground(p.Fg).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent).Padding(0, 1)

	s.Title = lipgloss.NewStyle().Foreground(p.Fg).Bold(true)
	s.Subtitle = lipgloss.NewStyle().Foreground(p.Muted)
	s.Muted = lipgloss.NewStyle().Foreground(p.Muted)
	s.Faint = lipgloss.NewStyle().Foreground(p.Faint)
	s.Accent = lipgloss.NewStyle().Foreground(p.Accent)
	s.Label = lipgloss.NewStyle().Foreground(p.Faint)
	s.Value = lipgloss.NewStyle().Foreground(p.Fg)
	s.Error = lipgloss.NewStyle().Foreground(p.Dead).Bold(true)

	s.Rule = lipgloss.NewStyle().Foreground(p.Border)

	// The agent pane is framed by a single left rule, not a box: a box costs
	// two rows and two columns of an area that belongs to the agent, and its
	// top edge competes with whatever header the agent draws.
	paneEdge := lipgloss.Border{Left: "│"}
	s.Pane = lipgloss.NewStyle().
		Border(paneEdge, false, false, false, true).
		BorderForeground(p.Border).
		PaddingLeft(1)
	s.PaneActive = lipgloss.NewStyle().
		Border(paneEdge, false, false, false, true).
		BorderForeground(p.Accent).
		PaddingLeft(1)

	// Modals stay boxed: they float over the frame and need a closed outline
	// to read as a separate surface.
	s.Modal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent)

	s.Card = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).Padding(0, 1)
	s.CardActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent).
		Background(p.SelectBg).Padding(0, 1)
	s.GroupLabel = lipgloss.NewStyle().Foreground(p.Faint).Bold(true)

	s.NavItem = lipgloss.NewStyle().Foreground(p.Muted).Padding(0, 1)
	s.NavItemActive = lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Padding(0, 1)
	s.Footer = lipgloss.NewStyle().Foreground(p.Faint)
	s.Key = lipgloss.NewStyle().Foreground(p.Fg).Bold(true)

	return s
}
