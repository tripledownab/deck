package store

// Connections between sessions: which pairs may see each other across the
// project boundary that otherwise separates them.

import "sort"

// Connection joins two sessions so each can see the other's work.
//
// A pair rather than a named set. Sessions on one project already see each
// other, so the case project scope cannot express is one repository's API
// changing while its consumer changes in another — and that case is two
// sessions. A set would need a name, a member editor, and a rule for what
// happens when the last member leaves, none of which that case asks for.
// Connecting A to B and A to C therefore lets A see both without making B and
// C visible to each other.
//
// It lives on State rather than as a field on Session so there is no two-way
// link to keep consistent: the pair is one record, and dropping it drops the
// whole relationship.
type Connection struct {
	A string `json:"a"`
	B string `json:"b"`
}

// joins reports whether the pair names these two sessions, in either order.
// Order is not meaningful — A asked and B was picked, which is history, not a
// direction — so every comparison goes through here rather than testing the
// fields and forgetting the second case.
func (c Connection) joins(x, y string) bool {
	return (c.A == x && c.B == y) || (c.A == y && c.B == x)
}

// Connect links two sessions and reports whether the link was new. Asking
// twice is not an error: the caller is expressing a state, not an increment.
func (s *State) Connect(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	for _, c := range s.Connections {
		if c.joins(a, b) {
			return false
		}
	}
	s.Connections = append(s.Connections, Connection{A: a, B: b})
	return true
}

// Disconnect drops the link between two sessions and reports whether there was
// one.
func (s *State) Disconnect(a, b string) bool {
	for i, c := range s.Connections {
		if c.joins(a, b) {
			s.Connections = append(s.Connections[:i], s.Connections[i+1:]...)
			return true
		}
	}
	return false
}

// disconnectAll drops every link a session holds.
//
// Called from RemoveSession rather than exported for the caller to remember.
// A pair naming a session that no longer exists is not dangerous — ids are
// eight random bytes, so nothing inherits one — but it is a record that
// accumulates and that every reader has to skip.
func (s *State) disconnectAll(id string) {
	out := s.Connections[:0]
	for _, c := range s.Connections {
		if c.A != id && c.B != id {
			out = append(out, c)
		}
	}
	s.Connections = out
}

// ConnectedTo lists the sessions linked to id, sorted. Sorted because the
// order links were made is not information anyone wants, and an unstable order
// makes a rendered list move under the cursor.
func (s *State) ConnectedTo(id string) []string {
	var out []string
	for _, c := range s.Connections {
		switch id {
		case c.A:
			out = append(out, c.B)
		case c.B:
			out = append(out, c.A)
		}
	}
	sort.Strings(out)
	return out
}
