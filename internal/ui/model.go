package ui

// The application model and the builders that configure it before the program
// starts.

import (
	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/store"
)

// Model is the whole application state.
type Model struct {
	styles styleSet
	theme  string // the active theme id, persisted in settings
	keys   keyMap
	state  *store.State

	// settings is held whole so that saving one preference cannot drop
	// another. It used to be written as store.Settings{Theme: id}, which was
	// only correct while there was exactly one field.
	settings  store.Settings
	agentCmd  string
	agentArgs []string

	// coord is the cross-session coordination server. Nil when it failed to
	// start: sessions still run, they just cannot see each other, and the
	// failure is reported rather than hidden.
	coord *coord.Coordinator

	screen screen
	width  int
	height int

	// Dashboard.
	focus     column
	projectIx int
	tabIx     int
	listIx    int

	// Session view.
	rows     []sidebarRow
	rowIx    int
	attached bool
	// armed is true between the prefix key and the command key that follows.
	armed bool

	// runners holds the live agent process per session id. A session with no
	// entry here has never been opened, or has been closed; that is a normal
	// state, not an error.
	runners map[string]*agent.Runner

	form     *form
	browser  *browser
	picker   *picker
	showHelp bool

	// notice is a one-line message in the footer; fault is a failure the user
	// must see, and it outranks the notice.
	//
	// Neither is auto-cleared. For fault that is deliberate — a failure the
	// user has not acknowledged must not scroll away. A notice lingering until
	// the next one replaces it is a weaker choice than it looks: the footer
	// keeps saying "stopped wily-crane-bbbb" long after that stopped being
	// news, and the keys hint it displaces is more useful by then. Clearing it
	// on the next keystroke would fix that, and would touch every screen, so
	// it is not a thing to change while passing through.
	notice string
	fault  error
}

// WithTheme applies a persisted theme id. An unknown id falls back to the
// default inside paletteFor, so a hand-edited settings file cannot leave the
// app unstyled.
func (m Model) WithTheme(id string) Model {
	m.theme = id
	m.styles = newStyles(paletteFor(id))
	return m
}

// WithSettings applies the persisted preferences.
//
// An -agent given on the command line wins for this run: it is an explicit
// instruction about this launch, where the setting is a memory of the last
// choice. Passing no flag leaves agentCmd empty and the setting decides.
func (m Model) WithSettings(s store.Settings) Model {
	m.settings = s
	if m.agentCmd == "" {
		m.agentCmd = s.Agent
	}
	return m.WithTheme(s.Theme)
}

// WithCoordinator attaches the cross-session coordination server.
func (m Model) WithCoordinator(c *coord.Coordinator) Model {
	m.coord = c
	return m
}

// New builds the program model.
func New(state *store.State, agentCmd string, agentArgs []string) Model {
	m := Model{
		styles:    newStyles(paletteFor(defaultThemeID)),
		theme:     defaultThemeID,
		keys:      defaultKeys(),
		state:     state,
		settings:  store.DefaultSettings(),
		agentCmd:  agentCmd,
		agentArgs: agentArgs,
		runners:   map[string]*agent.Runner{},
	}
	m.rebuildRows()
	return m
}

// FocusProject puts the dashboard cursor on the project registered at path.
// An empty or unknown path leaves the selection alone, so launching outside a
// repository is not an error.
func (m Model) FocusProject(path string) Model {
	if path == "" {
		return m
	}
	for i := range m.state.Projects {
		if m.state.Projects[i].Path == path {
			m.projectIx = i
			break
		}
	}
	return m
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
