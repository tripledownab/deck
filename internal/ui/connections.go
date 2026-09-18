package ui

// Connecting a session to one on another project: choosing the far end, and
// keeping the coordinator's picture in step with the store's.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/store"
)

// syncConnections hands the coordinator the whole set of links.
//
// The whole set every time, not the one that changed. The store owns the
// document and the coordinator owns the live picture; a coordinator that
// applied deltas would be a second copy of the same state, free to drift the
// moment one update is missed. Sending everything makes a missed update
// impossible to observe.
func (m Model) syncConnections() {
	if m.coord == nil {
		return
	}
	links := make([]coord.Connection, 0, len(m.state.Connections))
	for _, c := range m.state.Connections {
		links = append(links, coord.Connection{A: c.A, B: c.B})
	}
	m.coord.SetConnections(links)
}

// openConnectPicker lists the sessions the selected one could be connected to.
//
// Only sessions on other projects are offered. Sessions on the same project
// already see each other, so listing one would offer a link that changes
// nothing — and a menu entry that does nothing is how a user learns to distrust
// the menu.
//
// Rows for sessions already connected read "connected", and choosing one
// disconnects. One key does both directions, which is what keeps this off a
// screen of its own.
func (m Model) openConnectPicker(sess *store.Session) (tea.Model, tea.Cmd) {
	if sess == nil {
		return m, nil
	}
	connected := map[string]bool{}
	for _, id := range m.state.ConnectedTo(sess.ID) {
		connected[id] = true
	}

	var rows []pickerRow
	for _, other := range m.state.Sessions {
		if other.ProjectID == sess.ProjectID {
			continue
		}
		desc := projectLabel(m.state, other.ProjectID)
		if other.Branch != "" {
			desc += " · " + other.Branch
		}
		if connected[other.ID] {
			desc += " · connected"
		}
		rows = append(rows, pickerRow{id: other.ID, label: sessionLabel(other), desc: desc})
	}
	if len(rows) == 0 {
		m.notice = "no sessions on another project to connect to"
		return m, nil
	}
	m.pickerSubject = sess.ID
	m.picker = newPicker(pickConnect, "Connect "+sessionLabel(*sess)+" to", rows, "")
	return m, nil
}

// connectPicked links or unlinks the chosen session and persists the result.
func (m Model) connectPicked(otherID string) (tea.Model, tea.Cmd) {
	sess := m.state.Session(m.pickerSubject)
	other := m.state.Session(otherID)
	if sess == nil || other == nil {
		return m, nil
	}
	if m.state.Disconnect(sess.ID, other.ID) {
		m.notice = "disconnected " + sessionLabel(*other)
	} else {
		m.state.Connect(sess.ID, other.ID)
		m.notice = "connected to " + sessionLabel(*other) +
			" on " + projectLabel(m.state, other.ProjectID)
	}
	// Saved before the coordinator is told. A link the agents can act on but
	// the store never recorded would vanish at the next restart, and the user
	// would have watched it work.
	if err := m.state.Save(); err != nil {
		m.fault = fmt.Errorf("save connection: %w", err)
		return m, nil
	}
	m.syncConnections()
	return m, nil
}
