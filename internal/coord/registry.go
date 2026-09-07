package coord

// Sessions arriving and leaving: what the coordinator knows about a live agent
// and what it forgets when that agent goes.

// Register adds or updates a live session.
func (c *Coordinator) Register(s Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[s.ID] = s
}

// Registered lists the session ids the coordinator currently knows about, so
// the caller can reconcile them against the processes that are actually alive.
func (c *Coordinator) Registered() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Unregister drops a session and every claim it held.
//
// Releasing on exit is why claims are in memory and not on disk: a claim held
// by a process that is gone is worse than no claim at all, because the next
// agent believes someone is working there.
//
// Connections are not dropped here, and that is the difference between them
// and everything else in this list. A claim, an inbox and a spend belong to a
// running process; a connection belongs to the session, survives its agent
// exiting, and is removed only when the store removes the session itself.
func (c *Coordinator) Unregister(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
	delete(c.claims, id)
	delete(c.inbox, id)
	delete(c.spend, id)
	for jid, j := range c.jobs {
		if j.From == id {
			delete(c.jobs, jid)
		}
	}
	c.status.clear(id)
}
