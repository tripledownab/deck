# Backlog

Work that is decided but not done, and the reasoning behind each. Things
deliberately *not* built are in `docs/architecture.md` under "Not built yet";
this file is only for work that should happen.

Last reviewed 2026-08-28.

## ~~1. Exact status from Claude Code hooks~~ — done 2026-08-23

Shipped. `coord` serves the endpoints on its existing listener and
`ui.statusOf` prefers a reported state over the heuristic. The config goes to
claude with `--settings`.

Two decisions worth keeping: the event's meaning travels in the **URL**, so no
hook body is ever parsed and a Notification subtype is distinguished by its
matcher rather than an undocumented field; and the handler answers 200 with an
**empty body**, because a hook's reply is a decision — `UserPromptSubmit` can
erase the prompt, `PostToolBatch` can stop the agentic loop, `Stop` can refuse
to let the turn end — and a status ping must not be able to do any of those.

It first shipped with too small a set and was wrong twice, both times leaving a
state that never cleared: a turn that ended in failure rather than normally,
and a permission that had been granted. Each is covered by an event that is
easy to leave out, and neither was visible from the one event that looked like
it should cover the whole turn.

The lesson is the same one `sweepExited` taught: a live test that exercises the
happy path proves the wiring, not the state machine. Both holes were found by
reading claude's own event table rather than by anything failing.

**Answered:** a per-session hooks config *merges* with the project's own
`.claude/settings.json` rather than replacing it. Verified, not inferred — two
`Stop` hooks, one from each source, both fired on one turn. Deck does not
silently disable a project's hooks.

## ~~2. The first commit~~ — done 2026-08-24

The initial commit, 83 files. Deliberately hash-free: an amend during the
pre-publication audit invalidated the hash this line used to name.

This unblocks what it existed for: a worktree needs a commit to branch from, so
`deck` can now offer an isolated session on Deck itself rather than only
the project directory.

## ~~3. Numbered session jumps~~ — done 2026-08-24

`^g 1`…`^g 9` jumps straight to a session, and arming the prefix numbers the
first nine cards so the targets are visible rather than counted.

It was not quite "a key handler and a bounds check". `Model.rows` interleaves
project headers, so counting rows lands on the wrong session or on a header;
the count has to be over sessions. Two smaller things came out of it: every
cursor landing now goes through `landOn`, so the rule about dropping the
attachment lives in one place instead of two, and the digit is read with the
one-rune guard that `form.typeInto` already needed — a pasted "12" arrives as a
single `KeyRunes` message and would otherwise jump to session 1.

## 4. Worktrees of a sub-repo, for collector projects

A collector project — a directory whose children are the repositories — can
only run sessions in the project directory today, because its root has no
branch to work from.

The useful version offers "isolated worktree of *<child>*", picking one of the
child repositories. That needs `store.Session` to record which sub-repo the
worktree came from, and `newSession` to derive the worktree path from that
rather than from the project. Not a large change, but it touches the session
record, so it wants doing deliberately rather than as a patch.

## 5. Parallel sessions are what expose an agent's own state writes

An agent that keeps one state directory for all of its processes has to merge
each write rather than rewrite the file whole. Running one agent at a time
never demands that, so an agent can ship without it and nobody notices.

Deck is what creates the condition, which is why the item is here. The fix
belongs in the agent, not in Deck, so nothing in this repository changes.
Any specific case goes to that agent's own tracker, not into a public note.

## ~~6. What the status hooks still do not cover~~ — settled 2026-08-24

Both gaps were settled by driving a real interactive `claude` under a PTY
(`live_claude_test.go`, opt-in like the rest).

**`◆ Needs you` is observed, not inferred.** A permission prompt in a live
session posts to the `waiting` endpoint. Breaking the matcher names fails the
test with `no waiting state arrived; last reported Working`, so it discriminates.
Two things the run cost to learn: a fresh directory raises a trust dialog that
swallows the first thing typed, and `text\r` in one write is read as a paste
and never submits. Both are encoded in the harness.

`idle_prompt` is not a usable trigger for this.

**Not every turn-end is observable.** One way of ending a turn produces nothing
to register, so the coordinator can keep saying "working" about a turn that has
finished. It is the third state that never cleared, after the failed turn and
the granted permission, and the only one with no event to subscribe to. Deck
compensates in the UI: `ui.staleWorkingReport` lets a pane that has been silent
for ten seconds override a "working" report, which bounds the damage of any
missing turn-end rather than just this one.

Ten seconds is measured, not guessed — the longest silence inside a real turn
was under a second — and a live test fails if that ever reaches half the
threshold. One gap remains in that guard: it covers an ordinary turn, not a
long **foreground** tool call, which is where a pane would most plausibly stop
being repainted. Producing one on demand is the obstacle: claude's own Bash
tool refuses a foreground `sleep` and backgrounds it instead, ending the turn
in seconds.

## 7. Clearing `Needs you` sooner

`PostToolBatch` fires when the batch resolves, so a long approved tool call
still reads `Needs you` while it runs. `PreToolUse` is the candidate, and the
question to answer first is whether it fires before or after the permission
dialog. Before, and it changes nothing — `Notification` would set "waiting"
straight after it.

Do not reject `PreToolUse` on the grounds that a stalled hook would cancel the
tool call. It would not: "A timed-out `command`, `http`, or `mcp_tool` hook
doesn't block the tool call. The call continues through the normal permission
flow." An earlier version of this note had that backwards, by applying the
SDK host-client behaviour to the http handler.

## 8. The footer notice never clears

`m.notice` is set in eleven places and cleared in none, so `stopped
wily-crane-bbbb` sits in the footer long after it stopped being news,
displacing the keys hint. Clearing it on the next keystroke is the fix and it
touches every screen, which is why it was not done while passing through.

## 9. bubblezone for mouse regions

Clickable nav items, tabs and session cards. Deferred originally because
`bubblezone`'s region sentinels are characters `lipgloss.Width` can miscount
and the pane needs exact cell counts — that reason has expired now the layout
is proven and covered by `TestFormNeverOverflowsTheModal` and the pane-width
assertions in `internal/agent`. Render functions are shaped so marking is a
one-line addition per region.

Lowest value of the set: the app is keyboard-first and nothing about it is
currently awkward without a mouse. It lived under "Not built yet" until the
deferral reason expired, and was listed in both places for a while — a deferred
item and a planned one are different claims.

## 10. Connections between sessions in a project

Sessions on one project can already read each other's work (`work`) and have it
reviewed by a spawned agent (`analyse`). Both are scoped to the project, which
is the same scope `Siblings`, `notes` and `message` use.

A **connection** would be a smaller grouping inside that: a set of sessions
that share context and can review each other, persisted as
`Connections []Connection` on `store.State` rather than as a field on
`Session`, so there is no two-way link to keep consistent. `Load` already
back-fills missing fields, so an older state file stays readable.

It was deliberately deferred rather than built first. A connection gates two
things — the shared log and the analysis — and until the analysis existed a
`Connection` type would have been stored, rendered and read by nothing. Now
that both exist, the question is answerable from use rather than from
prediction: **is project scope actually too coarse?** With three or four
sessions on one repository it is not, and a grouping inside it would add a
concept without removing a problem.

The case that project scope genuinely cannot express is a connection *across*
projects — an API changing in one repository while its consumer is updated in
another. `Siblings` excludes that by construction. If connections are built,
that is the motivating case, and it inverts the framing: a connection is not a
narrowing of the project, it is an escape from it.

One question decides the shape and should be answered before any code: does a
connection **narrow the shared notes log**, or only gate the analysis?
Narrowing changes behaviour every sibling relies on today; gating is additive.

## 11. Live token counts while a review runs

The sidebar shows elapsed time while a spawned analysis is in flight and the
exact cost once it lands, because the figures arrive only in the final result
envelope. Showing them as they accumulate needs `--output-format stream-json`
with `--include-partial-messages`, which is a parser rather than a field read.

Worth doing only if a review ever runs long enough that watching the number
move tells you something a spinner does not. The measured runs so far finish in
a few seconds.

## ~~12. A public-repo notice a cloner will meet~~ — done 2026-08-28

Shipped as **This tree is public** in `docs/architecture.md`, placed before
`## Overview` rather than at the end: it governs every section below it, and a
rule met after 600 lines is a rule met too late.

The decision it was waiting on was how much of the uncommitted working
instructions belong in the published tree. The answer drawn here is *the rule,
never the measurement*. What a contributor must obey travels — write for a
stranger, fixtures count, a push cannot be recalled. What only describes this
machine or this history stays out, which is the same distinction the rule
itself asks a contributor to make. So the section gives the mechanism — an
object survives a force-push — and stops there.
