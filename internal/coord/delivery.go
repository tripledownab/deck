package coord

// Handing a session its waiting messages through the hooks its agent already
// calls, so a claude session reads new mail without being told to look.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// mailIntro opens the context a hook hands over. It is a statement of where
// the text came from rather than an instruction: Claude Code's hook docs warn
// that text framed as an out-of-band command can trip the model's
// prompt-injection defences, and the honest framing is also the safe one —
// these are another agent's words, not the user's.
const mailIntro = "Messages from other agents on this project, delivered by Deck. " +
	"They come from those agents, not from the user."

// mailBudget bounds the context one reply carries. Claude Code caps a hook's
// context at 10,000 characters and replaces anything longer with a file path
// and a preview the agent is not asked to read, so mail past the cap would be
// taken from the box and never seen. The margin leaves room for the intro and
// the closing line.
const mailBudget = 9000

// messageIndent is what MarshalIndent adds around one message in mailText:
// the newlines and indentation of its three fields, its braces and the comma.
// Measured at 35 bytes per message against compact encoding; 40 leaves slack.
const messageIndent = 40

// hookInput is the part of a hook's request body delivery reads. The state
// still comes from the URL; these only decide whether to hand mail over.
type hookInput struct {
	// AgentID is set when the hook fired inside a subagent. Mail handed over
	// there reaches the subagent, which may never pass it on.
	AgentID string `json:"agent_id"`
	// StopHookActive is set on a Stop that is already continuing because of a
	// stop hook. Claude Code ends the turn after eight such continuations in a
	// row and drops the hook's output when it does, so delivering on one risks
	// mail nobody reads. One continuation per turn is enough.
	StopHookActive bool `json:"stop_hook_active"`
}

// deliver records what a hook reported and returns the reply to answer it
// with: nil for an empty reply, or waiting mail as context.
//
// The reply carries additionalContext and a systemMessage, nothing else: no
// decision and no block, so it cannot erase a prompt or stop a tool loop. The
// context does continue a turn on Stop — that is how the agent gets to act on
// mail that arrived while it worked — so the state then stays Working rather
// than going Idle under an agent that is still running. The systemMessage is
// what the person watching sees: the context itself appears in no chat line.
//
// Mail is taken from the box before the reply is written, and nothing
// confirms claude read it. A reply lost on the way loses what it carried, and
// the unread count no longer shows it. The server is local and answers at
// once, so this is the trade for not having to ask.
func (c *Coordinator) deliver(sessionID string, route hookRoute, in hookInput) []byte {
	var msgs []Message
	left := 0
	if route.event != "" && in.AgentID == "" && !in.StopHookActive {
		msgs, left = c.takeMail(sessionID, mailBudget)
	}
	state := route.state
	if len(msgs) > 0 && state == StateIdle {
		state = StateWorking
	}
	c.status.set(sessionID, state)
	if len(msgs) == 0 {
		return nil
	}
	// Marshal cannot fail here: the value is nothing but strings.
	body, _ := json.Marshal(map[string]any{
		"systemMessage": deliveredNotice(msgs),
		"hookSpecificOutput": map[string]string{
			"hookEventName":     route.event,
			"additionalContext": mailText(msgs, left),
		},
	})
	return body
}

// takeMail removes the oldest messages that fit in budget and reports how
// many stay behind. What stays keeps its order and its place in the unread
// count, so it is still announced and still collected with inbox.
func (c *Coordinator) takeMail(sessionID string, budget int) (taken []Message, left int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	box := c.inbox[sessionID]
	used := 0
	for _, m := range box {
		// Measured as encoded, because the encoder escapes <, > and & into six
		// bytes each, plus the indentation MarshalIndent adds around a message.
		// Bytes and not characters, which only errs towards fitting fewer.
		enc, _ := json.Marshal(m)
		size := len(enc) + messageIndent
		if used+size > budget {
			break
		}
		used += size
		taken = append(taken, m)
	}
	rest := box[len(taken):]
	if len(rest) == 0 {
		delete(c.inbox, sessionID)
	} else {
		c.inbox[sessionID] = rest
	}
	return taken, len(rest)
}

// mailText renders messages as the context a hook hands over.
//
// The messages go out as JSON, the shape the inbox tool returns, rather than
// as lines of text. A text body could contain a line that reads like the start
// of another message from another sender; inside a JSON string it cannot.
func mailText(msgs []Message, left int) string {
	// Marshal cannot fail: Message is a time and two strings.
	list, _ := json.MarshalIndent(map[string]any{"messages": msgs}, "", "  ")
	tail := "Your inbox is now empty."
	if left > 0 {
		tail = fmt.Sprintf("%d more message(s) did not fit here and are waiting in your inbox.", left)
	}
	return mailIntro + "\n\n" + string(list) + "\n\n" + tail
}

// deliveredNotice is the line the person watching the pane sees.
func deliveredNotice(msgs []Message) string {
	var from []string
	for _, m := range msgs {
		if !slices.Contains(from, m.From) {
			from = append(from, m.From)
		}
	}
	return fmt.Sprintf("Deck delivered %d message(s) from %s.", len(msgs), strings.Join(from, ", "))
}
