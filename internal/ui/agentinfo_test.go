package ui

// The footer naming the binary a session started and the claude behind it,
// against fake programs on a PATH the test controls. A real claude on the
// machine running the tests would otherwise decide what they see.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tripledownab/deck/internal/store"
)

// fakeClaude is a claude that prints a version and otherwise stays up, as
// the real one does in a pane. Absolute paths, because PATH holds only bin.
const fakeClaude = "#!/bin/sh\nif [ \"$1\" = --version ]; then echo '2.0.9 (Claude Code)'; exit 0; fi\nexec /bin/sleep 30\n"

// fakeBin writes executables into a fresh directory and makes it the whole
// PATH, so nothing installed on the machine can answer instead.
func fakeBin(t *testing.T, files map[string]string) string {
	t.Helper()
	bin := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return bin
}

// startedWith starts a session running command and returns the model, and the
// notice once the version check has answered.
func startedWith(t *testing.T, command string) (Model, string) {
	t.Helper()
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "s", Agent: command, Dir: t.TempDir()})
	m := sized(New(st, command, nil), 120, 30)
	m.screen = screenSession
	t.Cleanup(func() { m.Close() })

	next, cmd := m.attach()
	m = next.(Model)
	if m.fault != nil {
		t.Fatalf("start: %v", m.fault)
	}
	if cmd == nil {
		return m, m.notice
	}
	after, _ := m.Update(cmd())
	return m, after.(Model).notice
}

// TestCathodeNamesTheClaudeThatAnswers is the case this exists for: the
// session runs cathode, and the claude cathode starts is the one that matters.
func TestCathodeNamesTheClaudeThatAnswers(t *testing.T) {
	bin := fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n", "claude": fakeClaude})

	_, notice := startedWith(t, "cathode")
	want := "claude 2.0.9 at " + filepath.Join(bin, "claude")
	if !strings.Contains(notice, filepath.Join(bin, "cathode")) || !strings.Contains(notice, want) {
		t.Errorf("notice = %q, want the cathode file and %q", notice, want)
	}
}

// TestClaudeNamesItsOwnVersion: the path is already the claude, so only the
// version is added.
func TestClaudeNamesItsOwnVersion(t *testing.T) {
	bin := fakeBin(t, map[string]string{"claude": fakeClaude})

	_, notice := startedWith(t, "claude")
	if want := filepath.Join(bin, "claude") + " (2.0.9)"; !strings.Contains(notice, want) {
		t.Errorf("notice = %q, want %q", notice, want)
	}
}

// TestAMissingOrBrokenClaudeIsSaid: either is the notice that should send
// someone to look, so neither is dropped.
func TestAMissingOrBrokenClaudeIsSaid(t *testing.T) {
	fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n"})
	if _, notice := startedWith(t, "cathode"); !strings.Contains(notice, "claude not found on PATH") {
		t.Errorf("with no claude: notice = %q", notice)
	}

	fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n", "claude": "#!/bin/sh\nexit 3\n"})
	if _, notice := startedWith(t, "cathode"); !strings.Contains(notice, "could not read the version") {
		t.Errorf("with a claude that fails: notice = %q", notice)
	}
}

// TestOtherAgentsAreNamedWithoutAVersion: Deck knows what --version means only
// for claude, so other agents get their path and no check at all.
func TestOtherAgentsAreNamedWithoutAVersion(t *testing.T) {
	bin := fakeBin(t, map[string]string{"codex": "#!/bin/sh\nexec /bin/sleep 30\n", "claude": fakeClaude})

	m, notice := startedWith(t, "codex")
	if notice != "started "+filepath.Join(bin, "codex")+" in "+m.state.Sessions[0].Dir {
		t.Errorf("notice = %q", notice)
	}
}

// TestALaterNoticeIsNotOverwritten: the check answers late by design, and by
// then the footer may say something newer.
func TestALaterNoticeIsNotOverwritten(t *testing.T) {
	fakeBin(t, map[string]string{"claude": fakeClaude})
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "s", Agent: "claude", Dir: t.TempDir()})
	m := sized(New(st, "claude", nil), 120, 30)
	m.screen = screenSession
	t.Cleanup(func() { m.Close() })

	next, cmd := m.attach()
	m = next.(Model)
	m.notice = "something newer"
	after, _ := m.Update(cmd())
	if got := after.(Model).notice; got != "something newer" {
		t.Errorf("the late version replaced a newer notice: %q", got)
	}
}
