package coord

// Directed messages between agents on one project.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Message is one directed message waiting for a session to collect it.
type Message struct {
	At   time.Time `json:"at"`
	From string    `json:"from"`
	Text string    `json:"text"`
}

// maxInbox bounds a session's undelivered mail. An agent that never collects
// should not grow without limit; the oldest go first.
const maxInbox = 50

// Send queues a message for one sibling by session name, or for every sibling
// on the project when to is empty. It returns the names it reached.
//
// Delivery is a mailbox the recipient collects, not a write into its terminal.
// Typing into a live pane would corrupt the input of an agent mid-turn, and
// submitting on its behalf would let one agent put instructions in another's
// prompt with no human in the loop. Pull keeps both problems away, at the cost
// of the recipient having to ask — which is why every tool result carries an
// unread count.
func (c *Coordinator) Send(fromID, to, text string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	me, ok := c.sessions[fromID]
	if !ok {
		return nil, fmt.Errorf("unknown session")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("message is empty")
	}

	msg := Message{At: time.Now(), From: me.Name, Text: text}
	var sent []string
	for id, s := range c.sessions {
		if id == fromID || s.ProjectID != me.ProjectID {
			continue
		}
		if to != "" && s.Name != to {
			continue
		}
		box := append(c.inbox[id], msg)
		if len(box) > maxInbox {
			box = box[len(box)-maxInbox:]
		}
		c.inbox[id] = box
		sent = append(sent, s.Name)
	}
	sort.Strings(sent)
	if to != "" && len(sent) == 0 {
		return nil, fmt.Errorf("no live session named %q on this project", to)
	}
	return sent, nil
}

// Collect returns a session's waiting messages and empties its box.
func (c *Coordinator) Collect(sessionID string) []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	msgs := c.inbox[sessionID]
	delete(c.inbox, sessionID)
	return msgs
}

// Unread is how many messages are waiting, for the UI and for the hint
// appended to every tool result.
func (c *Coordinator) Unread(sessionID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.inbox[sessionID])
}
