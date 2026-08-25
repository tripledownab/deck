# Monitoring Deck from a phone

Status: **design note, nothing built.** Written 2026-08-23.

The goal is to see, from a phone, what the agents are doing: which sessions are
running, which are working versus idle, what they have claimed, what they have
recorded. Monitoring, not driving — see [Read-only is an
invariant](#1-read-only-is-an-invariant-not-an-intention) for why that
distinction is load-bearing rather than a scoping convenience.

## What already exists

Most of the data model is built and tested. What is missing is a way to ask for
all of it at once, and a surface to serve it on.

| State | Where it lives | Reachable how |
|---|---|---|
| Sessions: id, project, name, title, branch, dir | `internal/coord` | `Siblings(id)` — **per session, from that session's view** |
| Claims | `internal/coord/claims.go` | `ClaimCount(id)`, or inside `Siblings` |
| Notes | `internal/coord/notes.go` | `Notes(id)` |
| Unread messages | `internal/coord/messages.go` | `Unread(id)` |
| **Status (Working / Idle / Exited)** | `internal/ui/app.go` — `Model.runners` | `agent.Runner.Status()` |
| **Rendered agent screen** | `internal/agent` | `agent.Runner.Render()` |

Two things follow from that table:

- **`coord` has no whole-picture accessor.** Every method answers "what should
  *this session* be told", which is the right shape for the MCP tools and the
  wrong one for a dashboard. A monitor needs a new snapshot method.
- **Status is not in `coord` at all.** It comes from the live `Runner`, which
  only `internal/ui` holds. So a snapshot has to be assembled by the UI, not by
  the coordinator.

`internal/coord` already runs an HTTP listener (`mcp.go`) bound to localhost.
That is the MCP endpoint for agents and should stay separate from a monitoring
surface — different audience, different auth, different lifetime.

## Three shapes

| | Reach | What you get | Cost / risk |
|---|---|---|---|
| **Push only** | Anywhere | Pings on Working→Idle, Exited, message waiting, via an ntfy / Pushover / Slack webhook | Almost none — no inbound surface |
| **LAN dashboard** | Same Wi-Fi | A page: projects, sessions, status, claims, notes, ages; auto-refreshing | One inbound listener on the machine |
| **Tunnelled dashboard** | Anywhere | The same page over Tailscale or Cloudflare Tunnel | The machine becomes reachable from outside the LAN |

**Recommendation: the LAN dashboard**, structured so push is a later addition
rather than a rewrite. "Monitor" in practice means *browse* — which session,
how long, holding what — and a notification cannot answer that. Reaching it
from outside the LAN is then a Tailscale config choice made outside Deck,
not code in it.

## Decisions to make before building

### 1. Read-only is an invariant, not an intention

Deck runs coding agents with full file access. Any endpoint that can reach
a PTY is remote code execution on the machine, over Wi-Fi. So the monitor must
be unable to write, structurally:

- The monitor receives a **snapshot value**, never a `*agent.Runner`. It has
  nothing to call.
- A test asserts no handler writes to a runner.

Adding "stop session" or "send a message" from the phone later is a **separate
decision with its own authentication**, not a small extension of this one.

### 2. Pane contents are the sensitive part

Session names, statuses and claims are dull. The *rendered agent screen* is
whatever the agent printed: file contents, source, an environment variable it
echoed, a key in a stack trace. Serving that on a shared network is a real
leak.

Default to metadata only. Put pane content behind `-monitor-panes`, so
including it is something you chose today rather than something you enabled
once and forgot.

### 3. Access control

A bare port is an open window into your work. Generate a token per run, print
it in the TUI, and require it:

```
http://<host>:7777/?t=<token>
```

Per-run rather than persisted, so closing Deck invalidates it and a URL
left in phone history stops working.

## Sketch

A new `internal/monitor` package, fed by the UI:

```go
// Snapshot is the whole picture at one instant. Values only — no live handles,
// which is what makes the monitor structurally read-only.
type Snapshot struct {
    Projects []ProjectView
    Taken    time.Time
}

type SessionView struct {
    Name, Title, Branch, Project string
    Status                       string    // Working | Idle | Exited | Closed
    Claims                       []string
    Unread                       int
    Started                      time.Time
    Pane                         []string  // empty unless -monitor-panes
}
```

- `ui.Model.Snapshot()` assembles it from `runners` + `coord`.
- `monitor.Serve(addr, token string, snap func() Snapshot)` serves the page and
  a JSON endpoint.
- `main.go` wires them, prints the URL and token on start.

Serving HTML by hand keeps the dependency list where it is; a phone browser
polling a small page every few seconds needs no framework.

## Answering questions and steering from the phone

Monitoring is read-only by design. Answering the agent's questions, and
redirecting its work, is a second capability — and it turns out to be far more
tractable than the PTY architecture suggests, because the signal does not come
through the PTY at all.

### Why the obvious routes do not work

- **Reading the pane.** Deck gets bytes. Deciding "the agent is asking
  something" would mean pattern-matching another program's UI, which breaks on
  its next release.
- **`--permission-prompt-tool`.** This is how cathode intercepts questions, and
  it is the right idea — but cathode runs `claude` in `-p` mode. The flag is
  absent from `claude --help` as of CLI 2.1.228, and cathode only ever uses it
  under `-p`. Not a route for an interactive PTY session.

### Hooks are the route

Claude Code hooks fire in interactive sessions and carry structured JSON.
Verified against the hooks reference:

| Hook | Fires | Use |
|---|---|---|
| `Notification` | Claude sends a notification; matchers include `permission_prompt`, `idle_prompt`, `agent_needs_input`, `agent_completed` | **"The agent wants you."** Fire-and-forget — output and exit code are ignored |
| `PermissionRequest` | A tool call needs a permission decision | **The answer channel.** Runs synchronously; decides through a `decision` object |
| `Stop` | Claude finishes responding; carries `last_assistant_message` | Exact turn boundaries. Can block, which continues the conversation |

Two details make this practical:

- **Hooks can be HTTP.** Handler types include `command`, `http` and
  `mcp_tool`. So a hook can point straight at Deck's own server — no shell
  scripts on disk, and Deck already writes a per-session config file, so it
  can write a per-session hooks config the same way.
- **`PermissionRequest` is synchronous, with a default 600s timeout**
  (customisable per handler). That is the window in which a phone can answer.

**Unverified:** the exact JSON schema of the `PermissionRequest` `decision`
object. The reference links a "PermissionRequest decision control" section that
did not come back in the fetch. Confirm the field names before writing the
handler — do not infer them from `PreToolUse`, whose
`hookSpecificOutput.permissionDecision` shape may differ.

### This also retires the status heuristic

Today `Working` means "printed bytes in the last 900ms" (`agent.activityWindow`),
which is a guess that goes wrong exactly when an agent thinks quietly. `Stop`
and `Notification` give real turn boundaries, so wiring hooks fixes the
[known friction](../README.md#known-friction) about inferred status as a side
effect — without the stream-json side channel previously assumed necessary.

Worth doing for the local TUI even if the phone never happens.

### What this costs

Answering permission prompts from a phone means **the phone can authorise file
writes and shell commands on the machine.** That is not monitoring with a reply
box; it is remote control. Consequences to design for:

1. **Authentication must be real.** A URL token is adequate for a read-only
   LAN page and is not adequate here.
2. **A blocking hook hangs the agent.** If the phone never answers, the default
   600s timeout stalls the session. Use a short timeout and fall through to
   claude's own terminal prompt, so an unanswered phone degrades to "answer it
   at the desk" rather than a wedged agent.
3. **Agent conflict.** With `-agent cathode`, cathode already owns approvals
   through its own MCP server. Deck must not install a competing handler;
   detect and skip.
4. **Steering is separate from answering.** Redirecting work means injecting a
   prompt. Deck can already write to any PTY, and hooks now tell it exactly
   when a session is idle instead of guessing — so "type this when it next goes
   idle" becomes reliable. Never write into a busy pane.

### Suggested order

1. Hooks for `Stop` and `Notification` → exact status in the TUI. No phone, no
   new risk, immediate benefit.
2. Read-only LAN dashboard, now showing real status.
3. Push on `agent_needs_input` — the phone learns without polling.
4. Answering, behind real auth, with a short hook timeout.
5. Steering, queued to the next idle boundary.

Each step is useful alone, and the risky ones come last.

## Open questions

- Bind address: `0.0.0.0` is the point, but should it be opt-in per run
  (`-monitor :7777`) rather than always on? Leaning **opt-in** — a flag you
  pass when you want it beats a listener you forgot about.
- How much scrollback in a pane view — last screen only, or history?
- Does the phone need the shared notes log, or is that a desk activity?
- Push later: which service? `ntfy.sh` needs no account and is a single POST,
  which fits the "small addition" test better than Slack or Pushover.
- Confirm the `PermissionRequest` `decision` object schema before building the
  answer channel.
- Does installing hooks per session conflict with a project's own
  `.claude/settings.json` hooks, or do both run? If a project already hooks
  `Stop`, Deck must add to it rather than replace it.
- `--bare` skips hooks entirely. Should Deck refuse to promise exact status
  when the user passes it through `-agent-args`?
