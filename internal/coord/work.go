package coord

// The work tool: what a sibling session has changed, so an agent can read it
// without asking that session for anything.

import (
	"fmt"

	"github.com/tripledownab/deck/internal/gitx"
)

// Work returns the changes made in a sibling's worktree.
//
// Reading the worktree rather than asking its agent is the point. It needs no
// cooperation, no delivery and no attention from the other session, so it
// works while that agent is mid-turn and interrupts nothing.
//
// It does not survive that agent exiting. sweepExited unregisters a session
// whose process has gone, and this reads the live registry, so the work of a
// finished session is unreachable even though its worktree is still on disk.
// Reaching it would mean reading the store instead, which is a decision about
// what a session means after its agent stops rather than a detail of this
// tool.
//
// The target is named, not identified: names are what the sessions tool
// publishes and what message already accepts, so an agent has only one kind of
// handle to learn.
func (c *Coordinator) Work(sessionID, target string) (map[string]any, error) {
	c.mu.Lock()
	me, ok := c.sessions[sessionID]
	if !ok {
		c.mu.Unlock()
		return nil, fmt.Errorf("unknown session")
	}
	var found *Session
	for id, s := range c.sessions {
		if id == sessionID || s.ProjectID != me.ProjectID || s.Name != target {
			continue
		}
		copied := s
		found = &copied
		break
	}
	// Unlocked by hand rather than deferred, unlike every other method here:
	// gitx.Diff shells out to git, and holding the coordinator's mutex across
	// a subprocess would stall every other agent for the length of a diff.
	c.mu.Unlock()

	if found == nil {
		return nil, fmt.Errorf("no live session named %q on this project", target)
	}
	// Refused rather than answered approximately. A shared project directory
	// holds everyone's edits at once, so a diff of it would credit this
	// session with work it may not have done.
	if !found.Isolated {
		return nil, fmt.Errorf("%s runs in the project directory, so its changes "+
			"cannot be told apart from any other work there", found.Name)
	}

	w, err := gitx.Diff(found.Dir, found.BaseRef)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", found.Name, err)
	}
	out := map[string]any{
		"session": found.Name,
		"title":   found.Title,
		"branch":  found.Branch,
		"base":    w.Base,
		"summary": w.Stat,
		"patch":   w.Patch,
	}
	if w.Truncated {
		out["truncated"] = true
	}
	return out, nil
}
