package coord

// Soft locks on paths, and the view of who holds what.

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Claim is a soft lock one session holds on a repo-relative path.
//
// Soft because nothing can actually stop another process writing the file. The
// value is in the answer to "is anyone else in here", which is the question no
// amount of written discipline can answer.
type Claim struct {
	Path      string    `json:"path"`
	Reason    string    `json:"reason,omitempty"`
	SessionID string    `json:"-"`
	Session   string    `json:"session"`
	At        time.Time `json:"at"`
}

// Siblings returns the other live sessions on the same project, with the
// claims each of them holds.
func (c *Coordinator) Siblings(sessionID string) []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	me, ok := c.sessions[sessionID]
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(c.sessions))
	for id, s := range c.sessions {
		if id == sessionID || s.ProjectID != me.ProjectID {
			continue
		}
		held := make([]string, 0, len(c.claims[id]))
		for _, cl := range c.claims[id] {
			held = append(held, cl.Path)
		}
		sort.Strings(held)
		out = append(out, map[string]any{
			"session": s.Name,
			"title":   s.Title,
			"branch":  s.Branch,
			"claims":  held,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["session"].(string) < out[j]["session"].(string)
	})
	return out
}

// Claim records an intent to work on paths, and reports anyone already there.
//
// Paths are normalised to repo-relative before comparison. Without that, two
// sessions claiming the same file never collide: each works in its own
// worktree, so the absolute paths differ.
func (c *Coordinator) Claim(sessionID string, paths []string, reason string) (granted []string, conflicts []Claim) {
	c.mu.Lock()
	defer c.mu.Unlock()

	me, ok := c.sessions[sessionID]
	if !ok {
		return nil, nil
	}

	held := map[string]Claim{}
	for id, list := range c.claims {
		if id == sessionID || c.sessions[id].ProjectID != me.ProjectID {
			continue
		}
		for _, cl := range list {
			held[cl.Path] = cl
		}
	}

	now := time.Now()
	for _, raw := range paths {
		p := relativeTo(me.Dir, raw)
		if p == "" {
			continue
		}
		if other, taken := held[p]; taken {
			conflicts = append(conflicts, other)
			continue
		}
		if c.holds(sessionID, p) {
			granted = append(granted, p) // already ours; claiming twice is fine
			continue
		}
		c.claims[sessionID] = append(c.claims[sessionID], Claim{
			Path: p, Reason: reason, SessionID: sessionID, Session: me.Name, At: now,
		})
		granted = append(granted, p)
	}
	return granted, conflicts
}

// Release drops claims. With no paths, it drops all of the session's claims.
func (c *Coordinator) Release(sessionID string, paths []string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	me, ok := c.sessions[sessionID]
	if !ok {
		return nil
	}
	if len(paths) == 0 {
		freed := make([]string, 0, len(c.claims[sessionID]))
		for _, cl := range c.claims[sessionID] {
			freed = append(freed, cl.Path)
		}
		delete(c.claims, sessionID)
		sort.Strings(freed)
		return freed
	}

	drop := map[string]bool{}
	for _, raw := range paths {
		if p := relativeTo(me.Dir, raw); p != "" {
			drop[p] = true
		}
	}
	var kept []Claim
	var freed []string
	for _, cl := range c.claims[sessionID] {
		if drop[cl.Path] {
			freed = append(freed, cl.Path)
			continue
		}
		kept = append(kept, cl)
	}
	c.claims[sessionID] = kept
	sort.Strings(freed)
	return freed
}

func (c *Coordinator) holds(sessionID, path string) bool {
	for _, cl := range c.claims[sessionID] {
		if cl.Path == path {
			return true
		}
	}
	return false
}

// ClaimCount is how many claims a session holds, for the UI.
func (c *Coordinator) ClaimCount(sessionID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.claims[sessionID])
}

// relativeTo normalises a claim path against the session's working directory,
// so the same file claimed from two different worktrees compares equal.
func relativeTo(dir, raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(p))
}
