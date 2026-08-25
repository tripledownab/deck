package coord

// The coordination server itself: one localhost listener serving every
// session, and the hook endpoint that reports turn state.

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// serverName is the MCP server name agents see. Tools arrive at the model as
// mcp__deck__<tool>.
const serverName = "deck"

// server is a minimal MCP endpoint over the Streamable HTTP transport.
//
// It is hand-rolled rather than pulled from an SDK, matching cathode's
// approvals server: the surface is five tools, and the dependency is not worth
// it. Implemented here is exactly the request/response half of the spec —
// initialize, tools/list, tools/call. If a future client gets stricter about
// the handshake (a GET SSE channel, Mcp-Session-Id round-tripping), this is
// the file to extend.
//
// One listener serves every session. The session id is the last path segment,
// so a call identifies its caller by the URL it was configured with; the agent
// never has to prove who it is, and cannot claim to be someone else without
// being handed that URL.
type server struct {
	c    *Coordinator
	ln   net.Listener
	http *http.Server
	base string
}

func newServer(c *Coordinator) (*server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	s := &server{c: c, ln: ln, base: "http://" + ln.Addr().String()}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/", s.handle)
	mux.HandleFunc("/hooks/", s.handleHook)
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.http.Serve(ln) }()
	return s, nil
}

func (s *server) urlFor(sessionID string) string { return s.base + "/mcp/" + sessionID }

// hookURL is the endpoint one hook posts to. The kind is the last segment, so
// the event's meaning travels in the URL and the body is never parsed.
func (s *server) hookURL(sessionID, kind string) string {
	return s.base + "/hooks/" + sessionID + "/" + kind
}

// handleHook records a reported state.
//
// It answers 200 with an empty body, because a hook's reply is a decision.
// Three of the five events Deck registers act on one: UserPromptSubmit can
// block the prompt and erase it, PostToolBatch can stop the agentic loop, and
// Stop can refuse to let the turn end. A status ping must not be able to do
// any of that, and the way to guarantee it is to say nothing at all.
func (s *server) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID, kind, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/hooks/"), "/")
	if !ok || sessionID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	state, known := hookPaths[kind]
	if !known {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.c.status.set(sessionID, state)
	w.WriteHeader(http.StatusOK)
}

func (s *server) close() error { return s.http.Close() }
