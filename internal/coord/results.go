package coord

// Building an MCP tool result, and appending the unread-mail hint that makes
// a pull-only mailbox visible.

import "encoding/json"

// withHint appends a line to a text result, leaving anything else untouched.
func withHint(res map[string]any, hint string) map[string]any {
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		return res
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		return res
	}
	text, ok := first["text"].(string)
	if !ok {
		return res
	}
	first["text"] = text + hint
	return res
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
}

func jsonResult(v any) map[string]any {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult("encode result: " + err.Error())
	}
	return textResult(string(b))
}

func errorResult(text string) map[string]any {
	r := textResult(text)
	r["isError"] = true
	return r
}
