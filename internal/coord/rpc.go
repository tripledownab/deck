package coord

// The JSON-RPC half of the MCP endpoint: the envelope types, the method
// switch, and the two ways a response can be framed.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/mcp/")
	if sessionID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var req rpcReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// A notification carries no id and expects no body back.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := rpcResp{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		version := "2025-06-18"
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &p) == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion // echo the client's version
		}
		w.Header().Set("Mcp-Session-Id", sessionID)
		resp.Result = map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": "0.1.0"},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolSchemas()}
	case "tools/call":
		resp.Result = s.callTool(sessionID, req.Params)
	default:
		resp.Error = &rpcErr{Code: -32601, Message: "method not found: " + req.Method}
	}

	// Streamable HTTP allows either a plain JSON body or an SSE stream. Honour
	// the client's Accept header: current claude advertises text/event-stream
	// and needs the SSE framing.
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		writeSSE(w, resp)
		return
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, resp rpcResp) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(resp)
	_, _ = w.Write(append(b, '\n'))
}

func writeSSE(w http.ResponseWriter, resp rpcResp) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	b, _ := json.Marshal(resp)
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
