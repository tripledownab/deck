package store

// Reading and mutating the loaded document.

import (
	"sort"
	"time"
)

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

// SessionsFor returns a project's sessions, newest first.
func (s *State) SessionsFor(projectID string) []Session {
	var out []Session
	for _, sess := range s.Sessions {
		if sess.ProjectID == projectID {
			out = append(out, sess)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
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
	s.Sessions = append(s.Sessions, sess)
	return &s.Sessions[len(s.Sessions)-1]
}

// RemoveSession drops a session from the store. It does not touch the
// worktree on disk; the caller decides that, because removing a worktree can
// destroy uncommitted work.
func (s *State) RemoveSession(id string) {
	out := s.Sessions[:0]
	for _, sess := range s.Sessions {
		if sess.ID != id {
			out = append(out, sess)
		}
	}
	s.Sessions = out
}
