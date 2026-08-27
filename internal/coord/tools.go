package coord

// The tools agents see. Answering a call is in dispatch.go.

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
			"name": "work",
			"description": "Read what another agent on this project has changed — a summary of " +
				"every file it touched, and the patch. It needs nothing from them and does not " +
				"interrupt them, so it works while they are mid-turn. Only sessions that are " +
				"still running can be read.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session": map[string]any{
						"type":        "string",
						"description": "Session name from the sessions tool.",
					},
				},
				"required": []string{"session"},
			},
		},
		{
			"name": "analyse",
			"description": "Have a separate agent review another session's work and report back. " +
				"Returns immediately with an id; collect the answer later with the analysis tool. " +
				"The reviewer is given the diff and cannot run or change anything.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session": map[string]any{
						"type":        "string",
						"description": "Session name from the sessions tool.",
					},
					"question": map[string]any{
						"type":        "string",
						"description": "What you want to know. Omit for a general review.",
					},
				},
				"required": []string{"session"},
			},
		},
		{
			"name": "analysis",
			"description": "Collect an analysis you started. Reports whether it is still " +
				"running, and when finished the answer along with what it cost.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "description": "The id analyse returned."},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "inbox",
			"description": "Collect messages other agents sent you. Reading empties the box.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}
