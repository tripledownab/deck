// Package agent runs one coding-agent process per session inside a
// pseudo-terminal and keeps a virtual terminal screen for it.
//
// Deck does not speak any agent protocol. It allocates a PTY, starts the
// configured command in the session's directory, and feeds the bytes into a
// terminal emulator whose screen the UI paints into a pane. That is what lets
// the same code host `claude`, `cathode`, or anything else the user prefers:
// the agent keeps its own UI, and Deck only owns the frame around it.
package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const (
	// Idle means the process is alive and has printed nothing recently.
	Idle Status = iota
	// Working means the process printed something within activityWindow.
	Working
	// Exited means the process is gone; Err says why.
	Exited
)

// scrollbackLines bounds the emulator's history. Ten thousand lines is
// cathode's default and about a megabyte of cells per session.
const scrollbackLines = 10000

// Config describes one agent process.
type Config struct {
	Command string   // executable, e.g. "claude"
	Args    []string // arguments, e.g. ["--permission-mode", "plan"]
	Dir     string   // working directory: the session's worktree or repo
	Width   int      // initial pane width in cells
	Height  int      // initial pane height in cells
}

// Runner owns one PTY, one child process, and one emulated screen.
type Runner struct {
	cfg  Config
	cmd  *exec.Cmd
	ptmx *os.File
	term *vt.SafeEmulator

	// termMu serialises everything that touches the emulator's cell buffer.
	//
	// SafeEmulator's own mutex is not enough: CellAt takes it, returns a
	// *uv.Cell pointing into the live buffer, and releases it. Reading
	// Content and Style off that pointer therefore happens unlocked, while
	// the parser may be writing the same cell. Every cell walk and every
	// mutation goes through this lock instead.
	termMu sync.Mutex

	// ptyMu guards the operations that need the raw file descriptor.
	//
	// os.File is safe for concurrent Read, Write and Close, but Fd() is not:
	// it reads the descriptor while Close is destroying it. pty.Setsize needs
	// Fd, and pump closes the PTY the instant the child exits — so a resize
	// landing on an agent that just quit reads a descriptor being torn down,
	// and the ioctl can end up aimed at a recycled one. On a 50ms UI tick that
	// collision is routine, not theoretical.
	ptyMu   sync.Mutex
	ptyGone bool

	mu        sync.Mutex
	lastWrite time.Time
	started   time.Time
	exited    bool
	exitErr   error
}

// Start launches the agent process attached to a new PTY.
func Start(cfg Config) (*Runner, error) {
	if cfg.Command == "" {
		return nil, errors.New("agent: no command configured")
	}
	if cfg.Width < 20 {
		cfg.Width = 20
	}
	if cfg.Height < 5 {
		cfg.Height = 5
	}
	if _, err := exec.LookPath(cfg.Command); err != nil {
		return nil, fmt.Errorf("agent %q not found on PATH: %w", cfg.Command, err)
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Dir = cfg.Dir
	// TERM tells the agent what it may emit; it must match what the emulator
	// below can parse. COLORTERM unlocks 24-bit colour in most TUIs.
	cmd.Env = append(ScrubbedEnv(), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(cfg.Height),
		Cols: uint16(cfg.Width),
	})
	if err != nil {
		return nil, fmt.Errorf("start %s in %s: %w", cfg.Command, cfg.Dir, err)
	}

	term := vt.NewSafeEmulator(cfg.Width, cfg.Height)
	term.SetScrollbackSize(scrollbackLines)

	r := &Runner{
		cfg:     cfg,
		cmd:     cmd,
		ptmx:    ptmx,
		term:    term,
		started: time.Now(),
	}
	go r.pump()
	go r.respond()
	return r, nil
}

// Stop ends the agent process. It asks politely, then insists.
//
// Closing the PTY is what makes an idle agent exit: claude reads stdin and
// treats EOF as "we are done". A busy one is signalled and, if it still has
// not gone, killed.
func (r *Runner) Stop() {
	if r.Status() == Exited {
		r.release()
		return
	}
	r.closePTY()
	if r.cmd.Process == nil {
		return
	}
	_ = r.cmd.Process.Signal(os.Interrupt)

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if r.Status() == Exited {
				r.release()
				return
			}
		case <-deadline:
			_ = r.cmd.Process.Kill()
			r.release()
			return
		}
	}
}
