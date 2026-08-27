package coord

// Answering a tool call: the arguments every tool can take, and what each one
// does. The list agents see is in tools.go.

import (
	"encoding/json"
	"fmt"
	"time"
)

type toolCall struct {
	Name      string `json:"name"`
	Arguments struct {
		Paths    []string `json:"paths"`
		Reason   string   `json:"reason"`
		Text     string   `json:"text"`
		To       string   `json:"to"`
		Session  string   `json:"session"`
		Question string   `json:"question"`
		ID       string   `json:"id"`
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

	case "work":
		w, err := s.c.Work(sessionID, p.Arguments.Session)
		if err != nil {
			return errorResult(err.Error())
		}
		return jsonResult(w)

	case "analyse":
		job, err := s.c.Analyse(sessionID, p.Arguments.Session, p.Arguments.Question)
		if err != nil {
			return errorResult(err.Error())
		}
		return jsonResult(map[string]any{
			"id":      job.ID,
			"session": job.Subject,
			"state":   job.State.String(),
			"advice": "Reviewing takes a minute or two. Get on with something else " +
				"and collect it with the analysis tool.",
		})

	case "analysis":
		job, err := s.c.Analysis(sessionID, p.Arguments.ID)
		if err != nil {
			return errorResult(err.Error())
		}
		out := map[string]any{"id": job.ID, "session": job.Subject, "state": job.State.String()}
		switch job.State {
		case JobRunning:
			out["running_for"] = job.Elapsed.Round(time.Second).String()
		case JobFailed:
			out["error"] = job.Err
		default:
			out["answer"] = job.Answer
		}
		if job.State != JobRunning {
			out["cost_usd"] = job.Cost
			out["tokens"] = job.Tokens
			out["took"] = job.Elapsed.Round(time.Second).String()
			out["session_spend_usd"] = s.c.Spend(sessionID)
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
