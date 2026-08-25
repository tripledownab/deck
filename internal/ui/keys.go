package ui

// The binding table and the help groups generated from it, so a rebind cannot
// drift from its own documentation.

import (
	"github.com/charmbracelet/bubbles/key"
)

// PrefixKey is the one key Deck takes away from the agent.
//
// A session pane hosts a live claude process, which wants esc, tab, shift+tab,
// ctrl+c, and the arrows. So navigation goes behind a tmux-style prefix and
// every other keystroke reaches the agent untouched. ctrl+g is the choice
// because readline leaves it alone (ctrl+a and ctrl+b are line motions claude
// uses) and it needs no AltGr on a Nordic layout (which rules out ctrl+]).
//
// Press it twice to send a literal ctrl+g through to the agent.
const PrefixKey = "ctrl+g"

type keyMap struct {
	// Chrome navigation. Live whenever no pane is attached.
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Enter key.Binding

	// Dashboard.
	SwitchCol  key.Binding
	NewSession key.Binding
	AddProject key.Binding
	Delete     key.Binding
	Theme      key.Binding

	// Command mode, reached through the prefix.
	Dashboard   key.Binding
	Sessions    key.Binding
	NewSessCmd  key.Binding
	StopSessCmd key.Binding
	ThemeCmd    key.Binding
	HelpCmd     key.Binding
	QuitCmd     key.Binding
	NextSess    key.Binding
	JumpSess    key.Binding
	Attach      key.Binding
	Detach      key.Binding

	Help key.Binding
	Quit key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "previous section")),
		Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next section")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open / attach")),

		SwitchCol:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch column (dashboard)")),
		NewSession: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
		AddProject: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add project")),
		Delete:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "close session")),
		Theme:      key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),

		Dashboard:   key.NewBinding(key.WithKeys("d"), key.WithHelp("^g d", "dashboard")),
		Sessions:    key.NewBinding(key.WithKeys("s"), key.WithHelp("^g s", "sessions")),
		NewSessCmd:  key.NewBinding(key.WithKeys("n"), key.WithHelp("^g n", "new session")),
		StopSessCmd: key.NewBinding(key.WithKeys("x"), key.WithHelp("^g x", "stop the agent")),
		ThemeCmd:    key.NewBinding(key.WithKeys("t"), key.WithHelp("^g t", "theme")),
		HelpCmd:     key.NewBinding(key.WithKeys("?"), key.WithHelp("^g ?", "this help")),
		QuitCmd:     key.NewBinding(key.WithKeys("q"), key.WithHelp("^g q", "quit")),
		NextSess:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("^g j/k", "next / previous session")),
		JumpSess: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("^g 1…9", "jump to that session")),
		Attach: key.NewBinding(key.WithKeys("enter", "i"), key.WithHelp("^g ↵", "attach to the pane")),
		Detach: key.NewBinding(key.WithKeys("esc", " "), key.WithHelp("^g esc", "detach from the pane")),

		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "this help")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// helpGroup is one titled block in the help modal.
type helpGroup struct {
	title string
	keys  []key.Binding
}

// helpGroups is the source the help modal renders from.
//
// It exists because the modal used to be a hand-written table beside these
// bindings, which is the same rule in two places: adding the theme key updated
// the bindings and left the modal silently out of date. Generating from the
// bindings means a new key is documented by existing, not by remembering.
//
// The bindings' help text carries the prefix for command-mode keys ("^g d"),
// because the key itself is just "d" — the prefix is context the binding
// cannot know.
func (k keyMap) helpGroups() []helpGroup {
	return []helpGroup{
		{"CHROME", []key.Binding{
			k.Up, k.Down, k.Left, k.Right, k.SwitchCol, k.Enter,
			k.NewSession, k.AddProject, k.Delete, k.Theme, k.Help, k.Quit,
		}},
		{"COMMAND — press " + PrefixKey + " first", []key.Binding{
			k.Dashboard, k.Sessions, k.NextSess, k.JumpSess, k.NewSessCmd, k.StopSessCmd,
			k.Detach, k.Attach, k.ThemeCmd, k.HelpCmd, k.QuitCmd,
		}},
	}
}
