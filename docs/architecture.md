# Architecture

How Deck is put together, and why the parts that look strange are the way they
are. Most of it was learned by getting it wrong once, so each section states
the trap as well as the rule. `README.md` is the user-facing guide.

## Overview

Deck is a Bubble Tea TUI that manages projects and sessions and hosts one
agent process per session inside a pseudo-terminal. It speaks **no agent
protocol**. It allocates a PTY, starts the configured command in the session's
directory, feeds the bytes to a terminal emulator, and paints that emulator's
screen into a pane. The agent keeps its own UI; Deck owns only the frame.

That split is why the agent is a **choice, not a build-time decision**:
`claude`, `cathode`, `codex`, `gemini`, or anything else that talks to a
terminal works unchanged.

Each session records the program it started (`store.Session.Agent`), and the
new-session form picks it. That is what keeps a running session honestly
labelled after the default changes, and it is why nothing reads the agent from
the model at attach time. `settings.Agent` remembers the last choice as the
default for the next form; `-agent` overrides it for one run and is the way to
name a program the form does not list. A session recorded before the field
existed is filled in once by `store.Load`, so no reader has to guess.

The list in `ui.agentChoices` is a menu, not a capability boundary. What
differs between entries is how much Deck can say: only claude reports turn
status through hooks, and only claude and cathode take a coordination config
(`ui.coordArgs`), so everything else shows the activity heuristic and cannot
see its siblings.

```
main.go            flags, state load, cwd registration, teardown after Run
internal/naming    scheming-hawk-jhgk names, branch names, slugs
internal/gitx      repo root, branch, worktree add
internal/termquery answers terminal queries for harnesses with no terminal
internal/store     projects + sessions, atomic JSON persistence:
                     store.go      the document, load and save
                     paths.go      every path under the state dir
                     query.go      reading and mutating the document
                     settings.go   persisted preferences
internal/agent     PTY + vt emulator per session  <- the subtle package
                     agent.go      Runner, Start, Stop
                     pty.go        the two pumps, writes, resize, teardown
                     render.go     the cell walk
                     status.go     working / idle / exited, and how quiet
                     env.go        what a hosted agent must not inherit
internal/coord     cross-session coordination, exposed to agents over MCP:
                     coord.go      types + session lifecycle
                     claims.go     soft locks and who holds what
                     messages.go   the inter-agent mailbox
                     notes.go      the shared log and its compaction
                     status.go     turn state reported by hooks
                     mcp.go        the listener and the hook endpoint
                     rpc.go        JSON-RPC envelopes and framing
                     tools.go      the tool schemas and dispatch
                     results.go    tool results and the unread-mail hint
internal/ui        the Bubble Tea program, split by job:
                     model.go      the model and its builders
                     app.go        Update, View — the shell
                     input.go      who receives a keystroke
                     keyroutes.go  where a key goes once modals decline it
                     keys.go       the binding table; ptykeys.go encodes to bytes
                     selection.go  cursor and list navigation
                     actions.go    forms; sessions.go and projects.go do the work
                     runner.go     agent lifecycle; agentargs.go builds its argv
                     dashboard.go  + projectlist.go / projectdetail.go / help.go
                     session.go    + chrome.go / sidebar.go / status.go / pane.go
                     form.go       the modal widget; formsession.go and
                                   formproject.go build the forms,
                                   formfields.go the pieces, forminput.go keys
                     browser.go    the directory explorer; dirlist.go lists it
                     picker.go     the list modal; picker_open.go opens it
                     theme.go      + palette.go / styles.go
probe/             debug harness: renders frames without a human present
```

`gitx` holds only what something calls. It briefly carried `IsDirty`,
`AheadBehind`, `HeadSubject`, `RemoveWorktree` and `DeleteBranch` because a git
wrapper "should" have them; all five had tests and no caller, which is how
dead code passes review. If a feature needs one, add it back with the feature.

### Relationship to cathode

`cathode` (https://github.com/tripledownab/cathode) is a single-session
harness for the same CLI. It drives `claude` over
bidirectional **stream-json** (`claude -p --input-format stream-json
--output-format stream-json --verbose`) and renders the transcript itself,
which is what gives it exact turn status, inline diffs, and the approvals pane.

Deck deliberately does not do that. A multi-session manager that also owned
the protocol would have to re-implement all of cathode inside itself. Instead
cathode is *one of the agents Deck can host* (`deck -agent cathode`).

Exact per-session status does **not** require copying that channel. Claude Code
hooks give real turn boundaries in an interactive session
(`coord.HooksConfigJSON`). An earlier version of this paragraph said a
stream-json side channel was the answer; it is the expensive one.

Which events, though, is the whole problem. `Stop` alone is a trap: it does not
fire when a turn ends in an API error, and granting a permission fires nothing
at all, so a three-event config reports two states that never clear. Check the
current event table in Claude Code's own hooks documentation before adding one.
It is more current than any summary here, and `docs/backlog.md` records what
those holes looked like.

**Esc has no event at all**, so a third case cannot be fixed by registering the
right hook. `ui.staleWorkingReport` is the answer: a pane silent for ten
seconds overrides a "working" report, because only "working" needs a later
event to clear it. Do not tighten that towards `agent.activityWindow` — it is a
staleness bound on the report, not a second opinion on the heuristic, and a
short one would reinstate the guessing the hooks were added to remove.

Verifying any of this needs an interactive session, not `-p`: a `-p` run raises
no permission dialog and has no keyboard to interrupt. `live_claude_test.go`
drives the real thing under a PTY, and encodes two traps — a fresh directory
raises a trust dialog that swallows the first thing typed, and writing
`text\r` in one go is read as a paste and never submits.

### What a hosted agent must not inherit (`agent.dropFromEnv`)

`agent.ScrubbedEnv` is the one place that decides the environment a spawned
agent runs with. Three variables are removed and each has its own reason.

`ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` are the billing constraint,
carried over from cathode: `claude` treats either as a bearer token, so one
present makes it silently bill the API instead of the logged-in plan. Do not
remove this to fix an auth problem; that is a deliberate design change.

`CLAUDE_CODE_CHILD_SESSION` is set inside a running Claude Code session and
marks its children as subsessions, which **turns transcript saving off**.
Deck is a plausible thing to launch from inside such a session, and every
agent it then hosted wrote no transcript — so `dirHasHistory` found nothing,
`--continue` never applied, and reopening an isolated session started fresh
without saying so. The sessions Deck hosts are top-level ones.

Match the whole variable name. This was a substring test against the dropped
names concatenated together, which silently ate any variable whose name was a
tail of one of them: a plain `TOKEN` or `KEY` never reached the agent.

Pinned by `TestScrubbedEnvDropsWhatAnAgentMustNotInherit`, which asserts both
halves — what goes and what stays.

### The reply pump is mandatory (`internal/agent`)

A terminal is bidirectional. Programs ask it questions — cursor position (DSR),
device attributes (DA), background colour (OSC 11) — and **block on the
answer**. `vt` queues its replies on an `io.Pipe`, and an `io.Pipe` write blocks
until something reads it. That write happens inside the parser, under the
emulator lock.

So `Runner.respond` is not a nicety: without it the first query an agent sends
deadlocks the session outright, holding the lock that every `Render` needs. It
also fixes a slow path — `termenv` waits **five seconds** for an OSC 11 reply
before falling back to `$COLORFGBG`, so an unanswered query costs that long
before any lipgloss-based agent draws its first frame.

`respond` keeps draining after the PTY closes rather than returning, for the
same reason: a pump that stopped reading would block the parser mid-write.

Anything that runs a terminal program under an emulator needs both halves of
this loop. Tests that drive `deck` itself (`smoke_test.go`, `probe/`) answer
these queries too — that is what `answerQueries` is for.

### Two locks in `internal/agent`, for two different hazards

`termMu` guards the emulator's cell buffer; `ptyMu` guards the raw file
descriptor. They are not interchangeable and both are load-bearing.

**`ptyMu`.** `os.File` is safe for concurrent `Read`, `Write` and `Close`, but
`Fd()` is not — it reads the descriptor while `Close` destroys it. `pty.Setsize`
needs `Fd`, and `pump` closes the PTY the instant the child exits, so a resize
landing on an agent that just quit races the teardown and the ioctl can end up
aimed at a recycled descriptor. On a 50ms UI tick that collision is routine.
Every close goes through `closePTY`, which is idempotent and sets `ptyGone`;
`Resize` skips the ioctl once it is set.

### SafeEmulator's mutex is not sufficient (`agent.termMu`)

`SafeEmulator.CellAt` takes the emulator lock, returns a `*uv.Cell` **pointing
into the live buffer**, and releases the lock. Reading `Content` and `Style`
off that pointer therefore happens unlocked, while the parser goroutine may be
writing the same cell — a data race that shows up as a garbled pane, not a
crash, so it will not announce itself.

`Runner.termMu` serialises everything that touches the cell buffer: `pump`'s
`Write`, the whole of `Render`'s cell walk, `Resize`, `Size`, and `release`.
Nesting is safe because every path takes `termMu` before the emulator's own
lock, never the reverse.

Any new method that walks cells must hold `termMu` for the whole walk, not just
the `CellAt` call. The detector only catches this intermittently — it took ten
runs to reproduce — so `make race` on a loop is the check, not a single pass.

### Emulator teardown is not Close (`agent.release`)

`Emulator.Close` writes the emulator's `closed` flag, and unlike every other
mutating method it is promoted straight from the embedded `Emulator` rather
than wrapped by `SafeEmulator`'s mutex. Calling it while the parser runs is a
data race the detector catches.

So `release` shrinks the screen and drops the scrollback instead. The cost is
one parked goroutine per stopped session; the scrollback was the actual weight.
Revisit if `SafeEmulator` grows a guarded `Close`.

### KeySpace is not 32 (`ui.ptySequences`)

Bubble Tea numbers the control keys by byte value — `KeyCtrlA` is 1, `KeyTab`
9, `KeyEnter` 13, `KeyBackspace` 127 — so `keyToBytes` ends with a fallback
that emits `byte(k.Type)` for the 0..127 range. **`KeySpace` is not in that
range.** It sits in the negative "Other keys" block next to the arrows, so the
fallback never saw it and every space typed into an agent was dropped:
`echo hello world` arrived as `echohelloworld`.

Nothing caught it for a while because forms use `bubbles/textinput`, which
handles `KeySpace` itself — so a form that accepted spaces proved nothing about
the PTY path. `TestEveryTypableKeyEncodes` now asserts that no key a user can
press encodes to nothing, and `smoke_test.go` types a spaced command into a
live agent. Extend the first when adding a binding; never infer a key's numeric
value from its name.

### Never dispatch on `msg.String()` where text is typed

Bubble Tea batches the runes of one read into a single `KeyRunes` message whose
`String()` is the typed text. So the word "up" is indistinguishable from the up
arrow. This bit twice:

1. Our own form dispatch — fixed by switching on `msg.Type`.
2. **`bubbles/textinput` itself**, which matches its `PrevSuggestion` binding
   with `key.Matches`. Typing a title containing "up" made the whole batch
   disappear. `form.typeInto` fixes the class by feeding text fields one rune
   at a time: a one-rune message stringifies to that character, and no binding
   is a bare printable character.

`TestFormKeepsWordsThatNameKeys` covers "up", "down", "end", "delete", "home",
"tab", and "space". Add to it rather than disabling individual bindings.

### Agent coordination (`internal/coord`)

Sessions are isolated by construction — separate worktrees, separate claude
transcripts — so two agents will happily refactor the same interface in
parallel and find out at merge time. `internal/coord` is the one channel
through that isolation: an in-process MCP server exposing `sessions`, `claim`,
`release`, `note`, and `notes`.

It is modelled on cathode's `approvals.go` (hand-rolled JSON-RPC over
Streamable HTTP, honouring the client's `Accept` header for SSE framing) with
two differences:

- **One listener for every session.** The session id is the last path segment
  of the URL, so a call identifies its caller by the config it was handed. An
  agent cannot present itself as another session without being given that URL.
- **No TUI round-trip.** Cathode's approve tool blocks on the update loop.
  These tools answer from shared state under a mutex, so there is no path
  where an HTTP handler waits on Bubble Tea.

Three rules the design rests on:

1. **Claims are normalised to repo-relative** (`relativeTo`). Each session
   works in its own worktree, so the same file has a different absolute path
   in each. Compare absolutes and two agents editing one file never collide —
   the whole mechanism becomes decorative. `TestClaimsCollideAcrossWorktrees`
   pins it.
2. **Claims live in memory and die with the session.** A claim held by a
   process that has exited is worse than no claim, because the next agent
   believes someone is working there. The deliberate paths call
   `releaseCoord`, but an agent can also leave on its own — `/exit`, a crash,
   a kill — and nothing calls back into the UI when it does. `sweepExited`,
   polled from the frame tick, reconciles `coord.Registered()` against the
   live runners and is what actually closes that hole. This shipped broken
   once: unregistering was wired only to the two deliberate paths, so a
   self-exited agent held its claims forever.

   The lesson generalises. `coord.TestUnregisterReleasesClaims` passed the
   whole time, because it calls `Unregister` directly — it proves the
   coordinator releases *when told*, not that anything tells it. A test for a
   mechanism is not a test for its wiring; `ui.TestSweepReleasesSelfExitedSession`
   drives `Update` for exactly that reason.
3. **Notes are append-only, capped on read, and trimmed on disk.** Several
   agents write concurrently, so a read-modify-write would silently drop
   entries. `maxNotes` bounds what a reader gets (an uncapped log is re-read
   and re-billed by every agent that asks); `compactAbove`/`keepNotes` bound
   the file, because append-only with no trimming grows for the life of the
   project. `AppendNote` holds the mutex across the write *and* the
   compaction — that is what stops the rewrite losing a line that landed
   between counting and renaming.
4. **Messages are a mailbox, not a keystroke injection.** Deck can write
   into any session's PTY, and using that for delivery is tempting and wrong:
   it corrupts the input of an agent mid-turn, and submitting on its behalf
   lets one agent put instructions into another's prompt with no human in the
   loop. Delivery is pull, and the cost — a recipient that never asks never
   learns — is paid by appending an unread count to every other tool result
   (`withHint`) and showing `✉ n` in the sidebar.

Wiring the agent is the one place Deck knows anything about the program it
hosts (`ui.coordArgs`): `claude` takes `--mcp-config`, `cathode` forwards a
second config with `-mcp`. Any other agent gets no config file at all.

`TestLiveClaudeSeesSiblings` drives a real `claude` against the server and is
the only test that proves config shape, transport, and tool naming actually
work together. It is opt-in because it spends a subscription turn:
`DECK_LIVE=1 go test -run TestLiveClaude ./internal/coord/`. Two traps it
encodes: plan mode blocks MCP tool calls outright, and the prompt must go over
stdin — with stdin closed, claude reports "input must be provided" and ignores
a positional prompt.

### The store is global, not per-directory (`main.registerCwd`)

One `state.json` holds every project and session, so all of them are reachable
from wherever `deck` runs. A session's agent runs in that session's own `Dir`,
which is independent of the launch directory — `deck` in project B can attach a
session belonging to project A, and its agent still runs in A's worktree.

**A project need not be a repository.** `ui.resolveProject` is the single place
that decides what path to store: inside a repository it collapses to the root,
so one repo cannot be registered twice through two of its subdirectories;
anything else is kept, because a directory that merely collects repositories is
a project too. Both the explorer (`browserKey`) and the typed path
(`addProject`) go through it — they used to duplicate the rule, and only one of
them enforced the repo requirement in the first place.

A collector has no root to branch from, so `gitx.AddWorktree` rejects it
explicitly. Without that check the unborn-HEAD branch below it reports "no
commits yet", which is true of a non-repository and explains nothing.
`newSessionForm` takes `canWorktree` and defaults to the project directory when
false; both choices stay on offer because the project field can change without
rebuilding the form.

`registerCwd` adds the launch repository if it is not already known and returns
its root so `ui.FocusProject` can select it. It also seeds a **collector** —
`gitx.HoldsRepos`, one level only, hidden directories skipped, and the home
directory refused outright. That last is a separate check, not a consequence of
the depth limit: a single checkout sitting directly in `$HOME` makes it a
one-level collector, so `deck` run from home would have registered it. The
comment here used to claim depth covered that case — it did not, and
`TestHoldsReposRefusesHome` now holds the line. Anything else is left alone; seeding every directory anyone ran
`deck` in would fill the dashboard with Downloads folders. It used to be gated on an empty
store, which meant cd-ing into a new repository showed every project except
that one. Do not reintroduce that gate; it reads as the store being
per-directory when it is not. `TestStoreIsGlobalNotPerDirectory` covers both
halves.

### The directory explorer lists directories only (`ui.browser`, `ui.dirlist`)

`a` opens an explorer that shows **directories, and symlinks that resolve to
directories** — nothing else. A project is a directory, so a file is a row
that can be neither entered nor chosen.

It used to wrap `bubbles/filepicker` in directory mode. That library lists
every entry and sorts on `os.DirEntry.IsDir`, which reports on the link rather
than its target and is therefore false for every symlink. A linked checkout
sank below every real directory and read as missing. Neither the filter nor
the sort is reachable from outside the library — `readDirMsg` and the `files`
slice it carries are unexported — so the listing is ours now.

Two consequences worth keeping:

1. **A symlink is resolved with `os.Stat`, not `os.Lstat`.** The question is
   what the link points at. A link to a file, and a link to nothing, are both
   dropped: neither can be entered and neither can be registered. Descending
   into a link lands in the target, so the path shown is where the files are.
2. **`esc` is not handled by the explorer at all.** It closes the modal, which
   is the caller's business; binding it to "up one directory" as well would
   leave the modal with no way out. `TestBrowserEscIsNotHandled` pins that.

`gitx.HoldsRepos` had the same `IsDir` defect and the same fix: a directory
whose children are symlinks to checkouts is a collector, and testing `IsDir`
alone hid it.

The read is synchronous. A listing is one `Stat` per entry, the cost
`resolveProject` already pays on every pick, and doing it inline keeps the
explorer free of the message round-trip an async read would need.

Validation lives in `browserKey`, not the form: the explorer is still on
screen and one keystroke from the right directory, so an error belongs there.
A pick that survives opens the project form with the path prefilled and focus
on the description.

### One list widget, two jobs (`ui/picker.go`)

`picker` is a modal list with a `pickerKind`. It serves the theme list and any
form field marked `pickable` — currently the project field. The kinds behave
differently on purpose: the theme picker previews by applying and persists on
commit, while a field picker floats over the open form, previews nothing, and
only writes the chosen value back into the field.

The form's own choice field renders options as a row of chips and falls back to
a one-at-a-time cycler when they overflow. That is right for two working-copy
options and wrong for a project list, which grows with the user's machine —
stepping through fifty entries with ←/→ is not navigation. `pickable` is the
opt-out, and Enter over such a field opens the list instead of advancing.

Two things that were wrong here and are worth not repeating: the cycler
indented a three-line bordered chip with a `"  "` string prefix, which shifts
only the first line and leaves the border ragged (use a style's `PaddingLeft`);
and `form.update` grew a fourth return value for the pick signal, so every
caller and test had to be updated — check `f.update(` call sites when it
changes again.

### Themes and the picker (`ui/theme.go`, `ui/picker.go`)

Twelve themes, sharing cathode's ids, labels and source colours so a name means
the same thing in both tools — but **not** a shared file. The role vocabularies
differ (cathode needs diff-gutter and "you"-label colours; Deck's SelectBg
is a dark tint where cathode's equivalent is a bright fill), so a common schema
would have to satisfy both forever to save an edit made twice a year. If that
changes, `docs/` is the place to record the decision, not a silent refactor.

Each theme states five source colours; the twelve palette roles are derived
from them (`PaletteFor`). Deriving rather than hand-picking twelve values for
each of the twelve themes is what keeps the three dim levels — `Border`,
`Faint`, `Muted` — equally separated in every theme; `TestDerivedDimLevelsAreOrdered` fails if a
blend ever inverts them.

`Palette` holds concrete colours, not `lipgloss.AdaptiveColor`. Every theme is
dark, and an AdaptiveColor with identical light and dark values claims an
adaptation it does not perform. A light theme is the reason to revisit that,
not a reason to work around it.

**The picker previews by applying**, so the frame behind the modal restyles as
the cursor moves. That means `m.theme` is overwritten on every keypress, so the
id to restore on cancel is captured on the picker when it opens
(`picker.restore`) — reading it off the model at cancel time returns the last
preview, which is a bug this shipped with once.

### Focus has to be visible (`ui.columnStyle`, `ui.cursorMarker`)

`tab` moves dashboard focus between the project list and the session list. It
first shipped changing nothing but the colour of the PROJECTS heading, which
read as a bug: the Overview tab passed `false` for every row's selection so it
never drew a cursor, and the unfocused column's cursor was hidden entirely, so
there was nothing to see move.

Three rules now hold, and a change that breaks any of them reintroduces the
same complaint:

1. The focused column draws its top border in the accent colour
   (`columnStyle`). This replaced the full-width rule under the header, so it
   costs no rows. A whole-column background was the alternative and loses: it
   has to be re-asserted on every padded cell and fights the terminal's theme.
2. Both columns keep a cursor at all times (`cursorMarker`) — accent `▸` when
   focused, dimmed `·` when not. Never render the unfocused cursor as blank.
3. `toggleColumn` refuses to focus an empty session list and says why. Focusing
   a list with no rows gives the arrows nothing to move and shows no cursor.

The footer names what `↑`/`↓` will move rather than saying "tab column".
Covered by `TestToggleColumnRefusesEmptyList` and
`TestCursorMarkerKeepsUnfocusedPosition`.

### The agent pane is framed by a rule, not a box

`Styles.Pane` / `PaneActive` draw a **left border only**, accent when attached
and subtle when detached. A box cost two rows and two columns of an area that
belongs to the agent, and its top edge competed with whatever header the agent
drew. The rule costs `paneChromeCols` (2) horizontally — one for the rule, one
for padding — and **nothing vertically**, which is why `paneSize` returns the
full `bodyH`.

`Styles.Modal` is the boxed style, used by the help screen and the form. Those
float over the frame and need a closed outline to read as a separate surface.
Do not reuse `Pane` for a modal or it loses three of its sides.

If the frame ever gains or loses a row or column here, change
`paneChromeCols` / `sessionChromeRows` with it — the emulator is sized from
`paneSize`, so a mismatch shifts the whole pane.

### Layout arithmetic

Every pane row must be an exact cell count or the emulator and the drawn frame
disagree and the border drifts. `ui.Model.layout` and `ui.Model.paneSize` are
the single source of those numbers — call them, don't recompute. `truncate`
and `clip` are ANSI-aware and enforce exact widths and heights; `ansi.Truncate`
counts its ellipsis inside the budget, which is what the layout needs.

Any harness that drives a program through a PTY must answer the terminal
queries it sends — `internal/termquery.Answer`, used by both `smoke_test.go`
and `probe/`. A silent harness looks like a hang: termenv waits five seconds
for an OSC 11 reply before giving up. Do not re-implement it locally; the two
copies that existed before it was extracted had already drifted by one query
(DA), so the two harnesses tolerated different programs.

Verify a layout change by rendering a real frame rather than reasoning about
it. `probe/` exists for that:

```bash
go build -o /tmp/probe ./probe
XDG_STATE_HOME=/tmp/st /tmp/probe -dir /repo -wait 6s \
  -keys 'n|a session title|\x13' -- ./deck -agent bash
```

Multi-byte glyphs make raw dumps look misaligned when they are not — measure
the border column in runes before believing a drift.

### Failures are reported, never absorbed

A failed worktree is never quietly downgraded to running in the project
directory: that would put an agent to work in a tree the user believed was
untouched. A corrupt `state.json` fails the launch rather than starting empty,
because starting empty invites duplicate worktrees over sessions we cannot see.
Closing a session leaves its worktree on disk and says where. `model.fault`
holds errors the user must see and is never auto-cleared.

Reporting is not the same as abandoning. `ui.formProblem` puts a failure *into*
the open form instead of dismissing it, so the user keeps what they typed and
can correct the choice that failed. An unborn HEAD is the worked example: a
freshly `git init`-ed repository has no commit for a worktree to check out, so
`gitx.AddWorktree` returns `ErrNoCommits` with the way out in the message, and
switching to the project directory is one `tab` away. Check `gitx.HasCommits`
before anything else that assumes a resolvable HEAD — git's own "fatal: invalid
reference: HEAD" is accurate and useless.

### An exited agent is a restart, not a dead end

`landOn` drops the attachment when the cursor moves to a session with no
process. `dropDeadAttachment` covers the other direction, where the cursor
stays put and the process leaves — `/exit`, a crash, or a key sequence the
agent treats as quit. It runs on the frame tick beside `sweepExited`, which
already reads `Status`.

Without it the model stayed attached, so every key was written to a dead PTY.
The first one was refused and surfaced `send to agent: process has exited`,
which reads as a fault in Deck rather than as the agent having quit — and
the fault line is never auto-cleared, so it stayed on screen.

The pane says what to do next, because the banner only said what had happened.
`↵` runs `attach`, which already restarts an exited runner rather than
refusing. The hint promises a resumed conversation only when one is coming:
`willResume` is the one predicate `agentArgsFor` reads to decide
`--continue`, so the sentence and the argv cannot disagree.

`willResume` is agent-aware for the same reason `coordArgs` is. `--continue`
is claude's flag, and `dirHasHistory` reads claude's transcript directory, so
a worktree claude once used would otherwise hand the flag to whatever ran
there next and have the pane promise a resume that cannot happen.

Deck does **not** restart the process by itself. An agent that exited on
purpose should stay exited, and one that is crashing would be respawned twenty
times a second by the same tick that noticed.

### Teardown ordering

`Model.Close` stops every agent and runs in `main` **after** `tea.Program.Run`
returns, never from the update loop — `Stop` waits on the child, and waiting
inside the loop stalls the loop that keeps its output draining. Cathode
documents the same constraint for `Engine.Close`.

## Keys

`ctrl+g` is the command prefix and the only key taken from the agent
(`ui.PrefixKey`). It is not `ctrl+a` or `ctrl+b` — readline line-motions
`claude` uses — and not `ctrl+]`, which needs AltGr on a Nordic layout.
`^g ^g` sends a literal `ctrl+g` through. Bindings live in `ui/keys.go` and the
help screen is generated from that table, so a rebind cannot drift from its own
documentation.

Two things about `^g 1`…`^g 9` generalise. It counts **sessions, not rows** —
`Model.rows` interleaves project headers, so the fourth row and the fourth
session are different things, and `jumpToSession` skips the headers the same
way `moveRow` does. And the digits appear in the sidebar only while the prefix
is armed, for the same reason `commandHint` does: a card numbered all the time
spends two columns of title on something you are not doing. Every cursor
landing goes through `landOn`, which is what keeps the attachment honest when
the target has no live process.

## Backlog

`docs/backlog.md` holds work that is decided but not done, with the reasoning
for each. `docs/phone-monitoring.md` is a design note for work that is *not* in
the backlog and has nothing built — a read-only LAN dashboard, then answering
and steering. It records why the monitor must take a value rather than a
`*agent.Runner`, and why pane contents are opt-in.

The section below is the opposite list: things deliberately not built. Do not
move an item from there into the backlog without a reason the note does not
already answer.

## Not built yet

- Dashboard tabs beyond Overview and Sessions. The reference portal shows Wiki,
  Tasks, Catalog, PRs, and more; those are views onto a service catalog a local
  tool cannot fill, so they are left out rather than drawn empty.
- `--continue` for non-isolated sessions. See `agentArgsFor`: a shared project
  directory may have been last used by another tool, and resuming someone
  else's conversation silently is worse than starting fresh.
