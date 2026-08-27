# Deck

A terminal workspace for parallel coding-agent sessions.

Deck manages projects and sessions and draws the frame around them. The
agent itself is a separate program running in a pseudo-terminal — `claude` by
default, or [`cathode`](https://github.com/tripledownab/cathode), or anything else that talks to a
terminal. Deck speaks no agent protocol, which is why the agent is just a
choice on the new-session form.

Two views:

- **Dashboard** — projects on the left, the selected project's sessions on the
  right, with status and age per session. `tab` moves keyboard focus between
  the two columns; the focused one carries an accent-coloured top border, and
  the other keeps a dimmed cursor so you can still see where it is.
- **Session** — a sidebar of sessions grouped by project, and a main pane that
  is the live agent terminal.

![Deck's dashboard: a project list on the left, and the selected project's
sessions with status and age on the right](assets/deck-dashboard.png)

![A session: the sidebar of sessions on the left, the agent in the main pane,
and an accent rule between them while attached](assets/deck-codex.png)

The pane hosts whatever program the session names — the one above is `codex` on
its first run, before it has been signed in.

Every project and session in these images is invented. `make demo` builds the
throwaway workspace they are taken against.

## Build and install

```bash
make build      # -> ./deck
make install    # -> ~/.local/bin/deck, same place as cathode
make test
make race       # required before merging changes to internal/agent
make help       # list every target
```

Override the install location with `PREFIX`, and `make install` warns if it is
not on your PATH:

```bash
make install PREFIX=/usr/local/bin
make uninstall
make watch      # rebuild + reinstall on save (needs entr)
```

Running it needs `claude` on PATH with `claude login` already done. Verify with
`claude` then `/status`: anything but the subscription route means you are
billing the API.

## Use

```bash
cd ~/Work/some-repo
deck                        # seeds this repo as the first project
deck -agent cathode         # override the remembered agent for this run
deck -agent claude -agent-args "--permission-mode plan"
deck -agent cathode -agent-args "-mode ask"
```

Running `cathode` as the agent works and is the reason Deck hosts a process
rather than speaking a protocol. One caveat for parallel sessions: every
cathode instance shares `$XDG_STATE_HOME/cathode`, and its session list is
rewritten whole rather than merged, so two cathode sessions closing together
can drop entries from each other's `ctrl+r` picker. Conversations themselves
are safe — those live in claude's own per-directory transcripts.

![The add-project browser, listing the repositories inside a directory it was
pointed at](assets/deck-add-project.png)

Launching inside a git repository registers it and puts the cursor on it.

**The store is global, not per-directory.** Every project and session is
visible from wherever you run `deck`, and a session's agent runs in that
session's own worktree regardless of where you launched from. So you can `cd`
into another project, run `deck`, and pick up a session belonging to the first
one.

**A project does not have to be a git repository.** A directory that only
collects them — several checkouts side by side, coordinated from the parent —
is a project in its own right, and an agent running there sees all of them at
once. Launching `deck` in such a directory registers it, the same as launching
inside a repository does.

Sessions in a collector run in the project directory; there is no repository at
its root to branch from, so that is the default and an isolated worktree is
refused with a message saying why.

`a` opens a directory explorer for directories you are not standing in. It
starts beside the selected project, so its siblings are already on screen:
`↑`/`↓` to move, `→` to open a directory, `↵` to pick one, `esc` to cancel.
Picking a directory that is already registered, or a path that is not a
directory at all, says so and leaves you where you were.

## Keys

One key is taken from the agent: **`ctrl+g`**, the command prefix. Everything
else reaches the agent untouched, so `esc`, `tab`, `shift+tab`, `ctrl+c`, and
the arrows keep working inside `claude`.

`ctrl+g` rather than `ctrl+a` or `ctrl+b`, which are readline line-motions
`claude` uses, and rather than `ctrl+]`, which needs AltGr on a Nordic layout.

| Chrome | |
|---|---|
| `↑`/`k` `↓`/`j` | move selection |
| `←`/`h` `→`/`l` | switch section |
| `tab` | switch column (dashboard) — projects ⇄ sessions |
| `↵` | open / attach |
| `n` | new session |
| `a` | add project |
| `x` | close session |
| `t` | theme picker |
| `?` | help |
| `q` | quit |

| Command — press `ctrl+g` first | |
|---|---|
| `^g d` | dashboard |
| `^g s` | sessions |
| `^g j` / `^g k` | next / previous session (stays attached if it is live) |
| `^g 1`…`^g 9` | jump straight to that session |
| `^g n` | new session |
| `^g x` | stop the agent |
| `^g esc` | detach from the pane |
| `^g ↵` | attach to the pane |
| `^g q` | quit |
| `^g t` | theme picker |
| `^g ?` | the full key list |
| `^g ^g` | send a literal `ctrl+g` to the agent |

Press `^g` on its own and the footer becomes this list, so the prefix teaches
itself and you never have to remember the second key. Holding it also numbers
the first nine sidebar cards, which is what makes `^g 1`…`^g 9` usable without
counting. The numbers count sessions, not rows, so they run straight through a
project heading.

## Sessions and worktrees

Opening a session asks where it should run:

- **Isolated git worktree** (default) — a new branch `session/<name>` checked
  out under the state directory. Parallel sessions on one repository never
  collide.
- **Project directory** — runs in the repository itself. Simple, but two
  sessions here fight over the working tree.

A repository with no commits yet cannot have a worktree — there is no commit to
check out. Deck says so and leaves the form open with your title intact, so
switching to **Project directory** is one `tab` and one `→` away.

The project field steps with `←`/`→` and opens the full list on `↵`, so it
stays usable whether you have three projects or ninety.

A project is listed under the **Name** you give it when you register it, which
defaults to the directory it sits in. `e` on the dashboard renames one; the
path is not editable there, because changing it makes a different project
rather than the same one under another name. The last field picks
the agent — see **Choosing the agent** below.

Session names are `scheming-hawk-jhgk`: two words you can say out loud plus a
suffix that makes the branch unique.

Closing a session stops the agent and forgets the session. It leaves the
worktree on disk, because it may hold uncommitted work. The footer says where.

## Agents coordinating

Sessions are isolated on purpose, which also makes them blind to each other.
Deck runs a small MCP server so agents on the same project can see what
their siblings are doing. Each session's agent is wired to it automatically
(`claude` and `cathode`; anything else you wire yourself with `-agent-args`).

## Choosing the agent

![The new-session form: project chips, a title, the working-copy choice, and
the Agent field with its four choices](assets/deck-new-session.png)

The new-session form has an **Agent** field — `claude`, `cathode`, `codex`,
`gemini` — and the last choice becomes the default for the next session. Each
session remembers what it started, so changing the default never relabels a
session that is already running. `-agent` overrides the default for one run and
takes any program that talks to a terminal, listed or not.

Only `claude` reports exact turn status, and only `claude` and `cathode` are
handed a coordination config. Anything else runs perfectly well in a pane, with
status inferred from output and no sibling awareness.

| Tool | What the agent gets |
|---|---|
| `sessions` | Who else is working on this project, and the files they hold |
| `claim` | Announce paths you are about to touch; returns anyone already there |
| `release` | Give the paths back |
| `note` | Append to the project's shared log |
| `notes` | Read what other agents recorded |
| `work` | Read what another session has changed — summary and patch |
| `message` | Send to one sibling by name, or to all of them |
| `inbox` | Collect messages sent to you |

Claims are **advisory** — nothing stops another process writing the file. The
value is answering "is anyone else in here", which no amount of written
discipline can. They are compared repo-relative, so the same file collides
across worktrees, and they are released when the session stops.

Messages are a **mailbox the recipient collects**, not a write into its
terminal. Typing into a live pane would corrupt the input of an agent mid-turn,
and submitting on its behalf would let one agent put instructions into
another's prompt with nobody watching. The cost of pulling is that the
recipient has to ask, so every other tool result carries an unread count.

The sidebar shows `⊙ n` for claims held and `✉ n` for messages waiting.

## Status

A `claude` session reports what it is doing rather than having it guessed.
Deck writes a per-session hooks config and passes it with `--settings`.
`UserPromptSubmit` and `PostToolBatch` open a turn, `Stop` and `StopFailure`
close one, and the `Notification` subtypes that block on a person —
`permission_prompt`, `idle_prompt`, `agent_needs_input`, `elicitation_dialog`,
`elicitation_url_dialog` — mean a human is wanted.

Registering fewer of them costs a wrong dot. A turn boundary is not always one
event, and some transitions are visible only through a later event rather than
one of their own, so too small a set leaves a session reporting a state that
never clears.

`Needs you` clears when the approved tool *finishes* rather than when you
approve it, so a long tool call still reads `Needs you` while it runs.
Narrowing that is in `docs/backlog.md`.

Not every way of ending a turn is observable. A pane that has gone quiet for
ten seconds therefore overrides a `Working` report, which covers any turn-end
that arrives without an event rather than one particular case.

| Dot | Meaning |
|---|---|
| `◉ Working` | a turn is running |
| `◆ Needs you` | a permission prompt, a question, or a turn that hit an API error |
| `◉ Idle` | the turn finished |
| `◍ Exited` | the process is gone |
| `○ Closed` | never opened, or stopped |

`Needs you` is the one the old heuristic could never show: an agent blocked on
a prompt prints nothing, so it looked idle.

The hooks post to the same localhost server the coordination tools use, with
the session id in the URL. Their timeout is 5 seconds rather than the 600s
default — if Deck is gone, a turn boundary should not stall the agent for
ten minutes.

Sessions run with `-agent cathode` keep the heuristic: cathode spawns its own
`claude` with its own arguments, so a `--settings` given to cathode would never
reach the process the hooks describe.

Notes live in `$XDG_STATE_HOME/deck/notes/<project>.jsonl`. They are
append-only because several agents write at once, capped at 50 on read, and
the file itself is trimmed to its newest 100 entries once it passes 400 — an
append-only log with no trimming grows for the life of the project.

## Themes

`t` opens the picker (`^g t` from an attached pane). The frame restyles as the
cursor moves, so you judge a theme by the app rather than by its name; `↵`
keeps it, `esc` puts back what you had.

Twelve themes, using the same ids and source colours as
[cathode](https://github.com/tripledownab/cathode): **Cinder** (default), BBS, Dracula, Nord,
Solarized Dark, Tokyo Night, Gruvbox Dark, One Dark, Monokai, Catppuccin Mocha,
GitHub Dark, Rosé Pine. A name means the same thing in both tools.

They are deliberately *not* shared through a common file. The two apps have
different role vocabularies — cathode needs diff-gutter and "you"-label colours
Deck has no use for, and Deck's selected-row tint is dark where
cathode's equivalent is a bright fill — so a shared schema would have to
satisfy both forever to save an edit made twice a year.

The choice persists in `$XDG_STATE_HOME/deck/settings.json`.

## State

Everything lives under `$XDG_STATE_HOME/deck` (default
`~/.local/state/deck`), which is where `cathode` keeps its state too:

```
state.json                  projects and sessions
settings.json               preferences: the chosen theme
worktrees/<project>/<name>  session worktrees
notes/<project>.jsonl       the shared agent log
sessions/<id>.mcp.json      generated coordination config per session
```

## Known friction

- **Claude Code asks you to trust each new worktree.** A fresh worktree is a
  new folder, so the trust prompt appears once per isolated session. Press `1`
  then `↵`. Deck does not pre-approve folders on your behalf.
- **Mouse is wheel-only.** Wheel events scroll the attached pane. Clickable nav
  items, tabs, and session cards need `bubblezone`, which is not wired in yet.
- **Status is a heuristic for agents that do not report.** `claude` sessions
  report real turn boundaries through hooks (see **Status** below). Anything
  else — `cathode`, or a custom agent — falls back to "printed something in the
  last 900ms", which is wrong exactly when an agent thinks quietly.

## Licence

Apache License 2.0. See [LICENSE](LICENSE).
