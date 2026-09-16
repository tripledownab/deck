# Backlog

Work that is decided but not done, and the reasoning behind each. Things
deliberately *not* built are in `docs/architecture.md` under "Not built yet";
this file is only for work that should happen.

Last reviewed 2026-09-07.

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

## ~~10. Connections between sessions~~ — done 2026-08-28

Shipped, and the entry that stood here answered its own question. It asked
whether a connection should **narrow** a project, and concluded that the case
project scope cannot express is the opposite one: an API changing in one
repository while its consumer changes in another. So a connection is an escape
from the project, never a subdivision of it, and the shared log question it
left open resolves the same way — nothing a sibling relies on today changed.

Four decisions are worth keeping.

**A pair, not a named set.** `store.Connection{A, B}`. Sessions on one project
already see each other, so the motivating case is exactly two sessions. A set
would need a name, a member editor and a rule for the last member leaving, and
none of that is asked for by the case that justified the feature. Connecting A
to B and A to C lets A see both without making B and C visible to each other.

**Claims deliberately do not widen.** Every other scoping test became `sees`;
`Claim` kept `ProjectID`. A claim is a repo-relative path, so two sessions in
different repositories both claiming `internal/api/client.go` would be reported
as colliding over a file they do not share, and an agent that meets one false
conflict stops trusting the mechanism. This is the exception most likely to be
tidied away by a later reader, which is why it has a test named for it.

**The read widens, the write does not.** A note still goes to the writer's own
project log. A reader gets that log merged with what connected sessions wrote
in theirs, filtered to those sessions — a connection joins two sessions, so
handing over the far project's whole log would publish the notes of every
session there.

**The coordinator is told the whole set, never a delta.** `SetConnections`
replaces. The store owns the document and the coordinator holds a copy; a copy
updated by deltas is free to drift the first time an update is missed, and the
drift is invisible.

Still open, and deliberately not built: connecting sessions whose agents are
not running. The registry is live, so a connection to a stopped session is
recorded and does nothing until it starts. That matches `work`, which cannot
read an exited session either, and it is the same underlying question — what a
session means after its agent stops.

## ~~11. Live token counts while a review runs~~ — done 2026-08-28

The run now uses `--output-format stream-json --verbose
--include-partial-messages` and `agent.RunClaude` takes a callback that fires
on every event carrying usage.

What a captured stream showed, and what the design follows from: **only the
output count moves.** The first event of a turn already carries the final
input, cache-read and cache-write figures — in one measured run, 15,888 read
and 7,954 written were known before a single word was generated. And **cost is
not in any event but the last**, so a run in flight can report what it is using
and not what it will cost. The sidebar therefore shows tokens while a review
runs and dollars once it lands, rather than a dollar figure that would sit at
zero for the whole run and read as free.

One format, not two. The non-live path could have kept `--output-format json`,
but a second parser is a second place for claude's field names to drift, and
the totals are the thing least affordable to get quietly wrong.

It also closed a gap the old code's own doc comment denied. `RunClaude`
promised to return an error "only when there is no accounting at all", while
`cmd.Output` turned any non-zero exit into an error and discarded the result
envelope with it. Reading stdout to the end before waiting means a result that
arrived is returned whatever the process does afterwards.

## 13. A shell as a session

Not every session wants an agent. Reaching a server, running a migration, or
watching a log is work that belongs beside the agents rather than in a separate
terminal.

Almost all of it already works: `agent.Start` runs whatever `store.Session.Agent`
names, `coordArgs` gives no coordination flags to a program it does not know,
and `willResume` refuses `--continue` to anything but claude. `deck -agent
/bin/zsh` is a working shell session today. What is missing is the menu entry,
and three decisions around it.

1. **Which command.** `$SHELL`, falling back to `/bin/sh`. It holds the login
   shell on both macOS and Linux and Deck inherits it from the terminal it was
   started in. The authoritative record is per-platform and needs a subprocess
   to read — `getent passwd` on Linux, Open Directory on macOS, where
   `/etc/passwd` holds only system accounts — to reproduce a value already in
   hand.
2. **`-agent-args` must not reach it.** `agentArgsFor` prepends them
   unconditionally. That is harmless while `-agent` and `-agent-args` are set
   together, and stops being harmless once a shell is on the menu.
3. **The choice must not stick.** Submitting the form writes the agent to
   settings as the next session's default, which is wrong for a one-off.

Numbered 13 rather than reusing 12: that number already names the public-tree
notice, and a closed entry should not change meaning.

## 14. Deleting a session, and the worktree it leaves

Closing forgets the record and keeps the worktree, which is the right default
and currently the only one. Nothing in Deck removes what it keeps, so thirty
closed **isolated** sessions are thirty trees under
`$XDG_STATE_HOME/deck/worktrees`, thirty `session/*` branches, and thirty
entries in the project's `git worktree list`. The only way out today is git by
hand. A session that ran in the project directory leaves nothing behind, which
is the distinction the fourth decision below turns on.

`x` opens a modal with both outcomes rather than growing a second key.
**Close** keeps the worktree and is the default. **Delete** removes the worktree
and the branch. The Delete row carries `gitx.Diff` against `BaseRef` — "3 files
changed, 41 insertions" or "no files changed" — because a confirmation that
states a fact is answerable and one that states a warning is not. Deck already
measures exactly this for `analyse`, so the figure costs nothing new.

Four decisions the code has to keep.

**The disk work comes first, and the record is forgotten only if it succeeded.**
A dropped record over a surviving worktree is an orphan the user can no longer
see, retry or name. The order is: stop the runner, unregister from the
coordinator, remove the worktree, delete the branch, then `RemoveSession` and
`Save`.

**Nothing is forced.** `git worktree remove` refuses a dirty tree and `git
branch -d` refuses unmerged commits. Both refusals are the answer, reported with
the path. A "delete anyway" row is the one affordance the dirty case cannot
afford, and the way out — commit it, or remove it by hand — fits in the notice.
Add force only if that refusal proves to be a real obstacle.

**The worktree is the gate, the branch is best effort.** A clean worktree whose
branch holds unmerged commits removes fine, and then `branch -d` refuses. The
session is gone and the work is not, which is the right outcome, so the notice
says the branch was kept and why.

**A non-isolated session has no worktree and no branch.** Its `Dir` is the
project directory itself. It skips the modal and closes as it does today, and
the delete path must never be reachable with one. `coord.workOf` already refuses
one for the neighbouring reason — a shared project directory holds everyone's
edits at once, so its changes cannot be told apart — and the same fact rules out
deleting anything on such a session's behalf.

The guard that came out of reading the handler shipped ahead of this, because it
was small and needed nothing from the modal. `x` fired whatever column had
focus, while `dashboardSession` resolves through `listIx`, so pressing it on the
projects list closed that project's newest session — a row the keyboard was not
driving. The row was never invisible: `cursorMarker` keeps a dimmed cursor on
the unfocused column deliberately. What was missing was the focus, and
`sectionLeft` already scoped ←/→ by it for the stated reason — the focused
column is drawn with an accent border, and a key that reaches across makes that
border a lie. `focusedSession` is where the rule lives now, and `c` goes through
it too. It matters to this entry because the modal must open on the session the
user pointed at, not on whichever one `listIx` happens to hold.

Deliberately not this: `x` on the projects column meaning "remove project". What
happens to that project's sessions and their worktrees deserves its own answer,
not one reached in passing inside a session delete.

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
