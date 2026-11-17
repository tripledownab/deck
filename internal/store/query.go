package store

// Reading and mutating the loaded document.

import (
	"slices"
	"sort"
	"time"
)

// TitleLimit is the most characters a session title may hold. The rename form
// stops typing there and the title tool refuses a longer one, so a title set
// either way fits the same sidebar card.
const TitleLimit = 120

// Project is a git repository Deck manages sessions for.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Session is one agent run against a project.
//
// Dir is where the agent process runs. For an isolated session that is a git
// worktree Deck created; otherwise it is the project path itself. Branch
// is set only for isolated sessions.
type Session struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Name      string `json:"name"`
	Branch    string `json:"branch,omitempty"`
	// BaseRef is the commit the worktree was created at. Recorded rather than
	// derived: the branch it came from moves on, and by the time anyone asks
	// what this session changed, comparing against that branch's tip would
	// attribute everyone else's work to it too.
	BaseRef   string    `json:"base_ref,omitempty"`
	Dir       string    `json:"dir"`
	Isolated  bool      `json:"isolated"`
	Agent     string    `json:"agent"`
	CreatedAt time.Time `json:"created_at"`
	// Order places the session in its project's list, lowest first. A new
	// session takes its negated creation time (orderFor), which is below the
	// value of every session created before it by the clock, so it lands first
	// as newest-first always did. Moving one only permutes the values a
	// project's sessions already hold, raising equal ones apart.
	Order int64 `json:"order,omitempty"`
}

// orderFor is the Order a session starts with. Zero is "not set", which is
// what a state file written before Order existed holds, and Load fills it in
// from the same rule, so such a file keeps the order it was shown in.
func orderFor(created time.Time) int64 {
	return -created.UnixNano()
}

// Project returns the project with the given id, or nil.
func (s *State) Project(id string) *Project {
	for i := range s.Projects {
		if s.Projects[i].ID == id {
			return &s.Projects[i]
		}
	}
	return nil
}

// Session returns the session with the given id, or nil.
func (s *State) Session(id string) *Session {
	for i := range s.Sessions {
		if s.Sessions[i].ID == id {
			return &s.Sessions[i]
		}
	}
	return nil
}

// ProjectByPath returns the project registered at the given path, or nil.
func (s *State) ProjectByPath(path string) *Project {
	for i := range s.Projects {
		if s.Projects[i].Path == path {
			return &s.Projects[i]
		}
	}
	return nil
}

// SessionsFor returns a project's sessions in their Order: as the user
// arranged them, with any session opened since at the top.
func (s *State) SessionsFor(projectID string) []Session {
	var out []Session
	for _, sess := range s.Sessions {
		if sess.ProjectID == projectID {
			out = append(out, sess)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// MoveSession puts a session at position to in its project's list, clamped
// to the list, and reports whether its position changed.
//
// The values the project's sessions hold are reassigned in their new order
// rather than recomputed, so a session opened later by the clock still takes
// a value below all of them. Equal values — a hand-edited file with no creation times — are
// raised to be distinct first, upwards only for the same reason; reassigning
// equal values would move nothing and still report a move.
func (s *State) MoveSession(id string, to int) bool {
	sess := s.Session(id)
	if sess == nil {
		return false
	}
	list := s.SessionsFor(sess.ProjectID)
	from := slices.IndexFunc(list, func(x Session) bool { return x.ID == id })
	to = max(0, min(to, len(list)-1))
	if from == to {
		return false
	}
	orders := make([]int64, len(list))
	for i := range list {
		orders[i] = list[i].Order
		if i > 0 && orders[i] <= orders[i-1] {
			orders[i] = orders[i-1] + 1
		}
	}
	moved := list[from]
	list = slices.Delete(list, from, from+1)
	list = slices.Insert(list, to, moved)
	for i := range list {
		s.Session(list[i].ID).Order = orders[i]
	}
	return true
}

// AddProject appends a project and returns it.
func (s *State) AddProject(p Project) *Project {
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.Projects = append(s.Projects, p)
	return &s.Projects[len(s.Projects)-1]
}

// AddSession appends a session and returns it.
func (s *State) AddSession(sess Session) *Session {
	if sess.ID == "" {
		sess.ID = NewID()
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now()
	}
	if sess.Order == 0 {
		sess.Order = orderFor(sess.CreatedAt)
	}
	s.Sessions = append(s.Sessions, sess)
	return &s.Sessions[len(s.Sessions)-1]
}

// RemoveSession drops a session from the store, and every connection it held.
// It does not touch the worktree on disk; the caller decides that, because
// removing a worktree can destroy uncommitted work.
func (s *State) RemoveSession(id string) {
	out := s.Sessions[:0]
	for _, sess := range s.Sessions {
		if sess.ID != id {
			out = append(out, sess)
		}
	}
	s.Sessions = out
	s.disconnectAll(id)
}
