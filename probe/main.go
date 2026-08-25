// Command probe runs ./deck under a pseudo-terminal, answers the terminal
// queries a real emulator would answer, and prints the resulting screen.
//
// Debug scaffolding for looking at frames without a human at the keyboard.
// Not part of the app.
//
//	go run ./probe -dir /tmp/repo -- ./deck -agent bash
//	go run ./probe -keys 'n|hello|\x13' -wait 3s -- ./deck -agent bash
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"github.com/tripledownab/deck/internal/termquery"
)

func main() {
	dir := flag.String("dir", "", "working directory for the child")
	keys := flag.String("keys", "", "pipe-separated key batches sent 700ms apart")
	wait := flag.Duration("wait", 3*time.Second, "how long to run before printing")
	cols := flag.Int("cols", 120, "terminal width")
	rows := flag.Int("rows", 40, "terminal height")
	raw := flag.Bool("raw", false, "print styled output instead of plain text")
	flag.Parse()

	argv := flag.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: probe [flags] -- <command> [args...]")
		os.Exit(2)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = *dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(*rows), Cols: uint16(*cols)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	defer ptmx.Close()

	term := vt.NewSafeEmulator(*cols, *rows)
	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				_, _ = term.Write(chunk)
				if reply := termquery.Answer(chunk); len(reply) > 0 {
					_, _ = ptmx.Write(reply)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Drain the emulator's own replies so its parser never blocks.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := term.Read(buf); err != nil {
				return
			}
		}
	}()

	if *keys != "" {
		go func() {
			for _, batch := range strings.Split(*keys, "|") {
				time.Sleep(700 * time.Millisecond)
				_, _ = ptmx.WriteString(unescape(batch))
			}
		}()
	}

	select {
	case <-done:
	case <-time.After(*wait):
	}

	out := term.Render()
	if !*raw {
		out = ansi.Strip(out)
	}
	fmt.Println(out)

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

// unescape expands the few escapes the -keys flag needs.
func unescape(s string) string {
	r := strings.NewReplacer(
		`\x13`, "\x13", // ctrl+s
		`\x07`, "\x07", // ctrl+g
		`\r`, "\r",
		`\e`, "\x1b",
		`\t`, "\t",
	)
	return r.Replace(s)
}
