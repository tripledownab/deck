package coord

// Connections: which sessions may see each other across the project boundary.
//
// Everything else in this package scopes to a project, because that is where
// sessions share a repository. A connection is the one exception, and it goes
// the other way: it does not narrow a project, it lets a session out of one.
// The case it exists for is an API changing in one repository while its
// consumer changes in another.

import "sort"

// Connection joins two sessions. It mirrors the pair the store persists, the
// way Session mirrors the one the store records: this package holds the live
// picture and does not read the document.
type Connection struct {
	A string
	B string
}

// SetConnections replaces the picture of who is linked to whom.
//
// Replaces rather than adds, so the caller can hand over the whole set after
// any change without working out what moved. It is indexed here rather than
// searched per call because the predicate runs inside the loop of every tool
// that lists sessions.
func (c *Coordinator) SetConnections(links []Connection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	peers := make(map[string]map[string]bool, len(links)*2)
	for _, l := range links {
		if l.A == "" || l.B == "" || l.A == l.B {
			continue
		}
		if peers[l.A] == nil {
			peers[l.A] = map[string]bool{}
		}
		if peers[l.B] == nil {
			peers[l.B] = map[string]bool{}
		}
		peers[l.A][l.B] = true
		peers[l.B][l.A] = true
	}
	c.peers = peers
}

// sees reports whether one session may see another: same project, or joined by
// a connection. Caller holds the lock.
//
// Claiming deliberately does not go through this. A claim is a repo-relative
// path, so two sessions in different repositories both claiming
// internal/api/client.go would collide on a name they do not share. A
// connection widens what a session can read, never the soft lock.
func (c *Coordinator) sees(me, other Session) bool {
	return other.ProjectID == me.ProjectID || c.peers[me.ID][other.ID]
}

// connectedNotes returns what the given sessions wrote in their own projects'
// logs.
//
// Filtered to those sessions by name, not merged whole. A connection joins two
// sessions, so handing over the other project's entire log would publish the
// notes of every session there, including ones nobody connected to. Each
// project's file is read once however many connected sessions live in it.
//
// A log that will not read is reported, not skipped. Notes already returns the
// error when the caller's own log fails, and one os.ReadFile failure must not
// have two answers depending on which project the file belongs to. The
// expected case — a project that has no notes yet — is not a failure at all:
// readNotes turns a missing file into an empty log, so what reaches the error
// here is a real fault. Silence would tell an agent that a connected session
// had written nothing, which is the one conclusion a connection exists to stop
// it drawing.
//
// Called without the lock: it reads files, and the caller has already taken
// the copy of the registry it needs.
func (c *Coordinator) connectedNotes(peers []Session) ([]Note, error) {
	if len(peers) == 0 {
		return nil, nil
	}
	byProject := map[string][]string{}
	for _, p := range peers {
		byProject[p.ProjectID] = append(byProject[p.ProjectID], p.Name)
	}
	var out []Note
	for projectID, names := range byProject {
		wanted := make(map[string]bool, len(names))
		for _, n := range names {
			wanted[n] = true
		}
		notes, err := readNotes(c.notesPath(projectID))
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			if wanted[n.Session] {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// connectedElsewhere lists the live sessions a session is joined to that are
// not on its own project, newest picture first read under the lock.
//
// A connected session whose agent has exited is not here, because Unregister
// drops it from the registry. That matches Work, which reads the live registry
// for the same reason: what a finished session left behind is a question about
// what a session means after its agent stops, not something a reader of this
// list should answer by accident.
func (c *Coordinator) connectedElsewhere(me Session) []Session {
	var out []Session
	for id, s := range c.sessions {
		if id == me.ID || s.ProjectID == me.ProjectID || !c.peers[me.ID][id] {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
