package store

import (
	"slices"
	"testing"
)

func TestConnectingTwiceIsOneLink(t *testing.T) {
	s := &State{}
	if !s.Connect("a", "b") {
		t.Fatal("the first connection was refused")
	}
	// The same pair the other way round is the same relationship. Recording it
	// twice would give Disconnect a link to remove and one to leave behind.
	if s.Connect("b", "a") {
		t.Error("connecting the same pair in reverse made a second link")
	}
	if len(s.Connections) != 1 {
		t.Errorf("connections = %v, want one", s.Connections)
	}
}

func TestConnectedToReadsBothWays(t *testing.T) {
	s := &State{}
	s.Connect("a", "b")
	s.Connect("c", "a")

	if got := s.ConnectedTo("a"); !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("a is connected to %v, want [b c]", got)
	}
	// The link is not transitive. b and c are each joined to a and to nothing
	// else, which is what makes a pair a pair rather than a group.
	if got := s.ConnectedTo("b"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("b is connected to %v, want [a]", got)
	}
}

func TestASessionCannotConnectToItself(t *testing.T) {
	s := &State{}
	if s.Connect("a", "a") {
		t.Error("a session connected to itself")
	}
	if len(s.Connections) != 0 {
		t.Errorf("connections = %v, want none", s.Connections)
	}
}

// TestRemovingASessionDropsItsConnections is why disconnectAll is called from
// RemoveSession rather than left to the caller. A pair naming a session that
// is gone is a record every reader has to skip, and the caller that forgets is
// the one nobody reviews again.
func TestRemovingASessionDropsItsConnections(t *testing.T) {
	s := &State{}
	a := s.AddSession(Session{ProjectID: "p1", Name: "swift-otter-aaaa"})
	b := s.AddSession(Session{ProjectID: "p2", Name: "wily-crane-bbbb"})
	s.Connect(a.ID, b.ID)

	s.RemoveSession(a.ID)

	if len(s.Connections) != 0 {
		t.Errorf("connections = %v, want none after the session went", s.Connections)
	}
	if got := s.ConnectedTo(b.ID); len(got) != 0 {
		t.Errorf("the surviving session is still connected to %v", got)
	}
}

func TestDisconnectReportsWhetherThereWasALink(t *testing.T) {
	s := &State{}
	s.Connect("a", "b")

	if !s.Disconnect("b", "a") {
		t.Error("disconnecting in reverse order found nothing")
	}
	if s.Disconnect("a", "b") {
		t.Error("disconnecting twice reported a second removal")
	}
	if len(s.Connections) != 0 {
		t.Errorf("connections = %v, want none", s.Connections)
	}
}
