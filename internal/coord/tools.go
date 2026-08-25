package coord

// The tools agents see, and the dispatch that answers a call.

import (
	"encoding/json"
	"fmt"
)

func toolSchemas() []map[string]any {
	pathList := map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Repo-relative paths. Absolute paths inside your worktree are converted.",
	}
	return []map[string]any{
		{
			"name": "sessions",
			"description": "List the other agents working on this project right now, " +
				"with the files each has claimed. Call this before starting work.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name": "claim",
			"description": "Announce that you are about to work on these paths. Returns any " +
				"that another agent already holds, so you can pick different work or " +
				"coordinate. Advisory: it does not lock the files.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"paths":  pathList,
					"reason": map[string]any{"type": "string", "description": "What you are doing there."},
				},
				"required": []string{"paths"},
			},
		},
		{
			"name":        "release",
			"description": "Give up claims when you are done. Omit paths to release all of yours.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"paths": pathList},
			},
		},
		{
			"name": "note",
			"description": "Append a durable note to this project's shared log, readable by " +
				"every agent on it. Use it for decisions and discoveries that outlive your session.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"text": map[string]any{"type": "string"}},
				"required":   []string{"text"},
			},
		},
		{
			"name":        "notes",
			"description": "Read this project's shared log — what other agents recorded. Call this at the start of a task.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name": "message",
			"description": "Send a message to another agent on this project, or to all of " +
				"them. They collect it with the inbox tool; it does not interrupt them.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to": map[string]any{
						"type":        "string",
						"description": "Session name from the sessions tool. Omit to send to every sibling.",
					},
					"text": map[string]any{"type": "string"},
				},
				"required": []string{"text"},
			},
		},
		{
			"name":        "inbox",
			"description": "Collect messages other agents sent you. Reading empties the box.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

type toolCall struct {
	Name      string `json:"name"`
	Arguments struct {
		Paths  []string `json:"paths"`
		Reason string   `json:"reason"`
		Text   string   `json:"text"`
		To     string   `json:"to"`
	} `json:"arguments"`
}

func (s *server) callTool(sessionID string, params json.RawMessage) map[string]any {
	var p toolCall
	if err := json.Unmarshal(params, &p); err != nil {
		return errorResult("bad arguments: " + err.Error())
	}

	res := s.dispatch(sessionID, p)

	// Mail is pull-only, so a recipient would otherwise never learn it had
	// any. Every result except the inbox's own carries the count, which makes
	// any coordination call the moment a message surfaces.
	if p.Name != "inbox" {
		if n := s.c.Unread(sessionID); n > 0 {
			res = withHint(res, fmt.Sprintf(
				"\n\n(%d message(s) waiting from other agents — call the inbox tool.)", n))
		}
	}
	return res
}

func (s *server) dispatch(sessionID string, p toolCall) map[string]any {
	switch p.Name {
	case "sessions":
		siblings := s.c.Siblings(sessionID)
		if len(siblings) == 0 {
			return textResult("No other agents are working on this project.")
		}
		return jsonResult(map[string]any{"sessions": siblings})

	case "claim":
		granted, conflicts := s.c.Claim(sessionID, p.Arguments.Paths, p.Arguments.Reason)
		out := map[string]any{"claimed": granted}
		if len(conflicts) > 0 {
			out["conflicts"] = conflicts
			out["advice"] = "Another agent is already working on the conflicting paths. " +
				"Pick different work, or leave a note explaining the overlap."
		}
		return jsonResult(out)

	case "release":
		return jsonResult(map[string]any{"released": s.c.Release(sessionID, p.Arguments.Paths)})

	case "note":
		if err := s.c.AppendNote(sessionID, p.Arguments.Text); err != nil {
			return errorResult(err.Error())
		}
		return textResult("Noted.")

	case "notes":
		notes, err := s.c.Notes(sessionID)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(notes) == 0 {
			return textResult("No shared notes yet for this project.")
		}
		return jsonResult(map[string]any{"notes": notes})

	case "message":
		sent, err := s.c.Send(sessionID, p.Arguments.To, p.Arguments.Text)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(sent) == 0 {
			return textResult("No other agents are on this project, so nobody received it.")
		}
		return jsonResult(map[string]any{"delivered_to": sent})

	case "inbox":
		msgs := s.c.Collect(sessionID)
		if len(msgs) == 0 {
			return textResult("No messages.")
		}
		return jsonResult(map[string]any{"messages": msgs})
	}
	return errorResult("unknown tool: " + p.Name)
}
