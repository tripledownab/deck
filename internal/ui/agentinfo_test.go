package ui

// The footer naming the binary a session started and the claude behind it,
// against fake programs on a PATH the test controls. A real claude on the
// machine running the tests would otherwise decide what they see.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// claudeWith is a fake claude whose --version runs body, and which otherwise
// stays up as the real one does in a pane.
func claudeWith(body string) string {
	return "#!/bin/sh\nif [ \"$1\" = --version ]; then " + body + "; fi\nexec /bin/sleep 30\n"
}

// TestTheCheckDoesNotHoldTheUI: starting returns before the check answers.
// The check runs as a command; run inline it would freeze every key and frame
// for as long as claude takes to print its version.
func TestTheCheckDoesNotHoldTheUI(t *testing.T) {
	fakeBin(t, map[string]string{"claude": claudeWith("/bin/sleep 2; echo 1.0; exit 0")})
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "s", Agent: "claude", Dir: t.TempDir()})
	m := sized(New(st, "claude", nil), 120, 30)
	m.screen = screenSession
	t.Cleanup(func() { m.Close() })

	began := time.Now()
	next, cmd := m.attach()
	t.Cleanup(func() { next.(Model).Close() })
	if took := time.Since(began); took > time.Second {
		t.Errorf("starting took %v; the version check ran on the UI's goroutine", took)
	}
	if cmd == nil {
		t.Error("no check was returned to run")
	}
}

// TestAWedgedCheckIsBounded, and one whose child holds the output pipe: a
// shell wrapper's own child keeps stdout open after the wrapper is gone, and
// the read must not wait for it.
func TestAWedgedCheckIsBounded(t *testing.T) {
	// Long enough for a fresh shell script to start and print on a loaded
	// machine — 300ms missed that a few times in fifteen runs — and short
	// enough to keep the test under its bound.
	saved := versionTimeout
	versionTimeout = 1500 * time.Millisecond
	t.Cleanup(func() { versionTimeout = saved })

	for name, body := range map[string]string{
		"wedged":       "exec /bin/sleep 30",
		"child waits":  "/bin/sleep 30",
		"child stays":  "/bin/sleep 30 & echo 2.0.9; exit 0",
		"silent child": "/bin/sleep 30 & exit 0",
	} {
		fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n", "claude": claudeWith(body)})
		began := time.Now()
		_, notice := startedWith(t, "cathode")
		if took := time.Since(began); took > 5*time.Second {
			t.Errorf("%s: the check took %v", name, took)
		}
		want := "could not read the version"
		switch name {
		case "child stays":
			want = "claude 2.0.9 at"
		case "wedged":
			want = "no answer within"
		}
		if !strings.Contains(notice, want) {
			t.Errorf("%s: notice = %q, want %q", name, notice, want)
		}
	}
}

// TestTheReasonForAFailureIsShown: "exit status 1" alone does not say why, and
// the line claude wrote to stderr usually does.
func TestTheReasonForAFailureIsShown(t *testing.T) {
	fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n",
		"claude": claudeWith("echo 'boom: node too old' >&2; exit 1")})
	if _, notice := startedWith(t, "cathode"); !strings.Contains(notice, "boom: node too old") {
		t.Errorf("notice = %q, want the reason claude gave", notice)
	}

	// An escape sequence in it reaches the footer as text, not as a command
	// to the terminal.
	fakeBin(t, map[string]string{"cathode": "#!/bin/sh\nexec /bin/sleep 30\n",
		"claude": claudeWith("printf '\\033[31mboom\\033[0m\\n' >&2; exit 1")})
	if _, notice := startedWith(t, "cathode"); strings.ContainsRune(notice, 0x1b) || !strings.Contains(notice, "boom") {
		t.Errorf("notice = %q, want the reason without control bytes", notice)
	}
}

// TestAClaudeThatPrintsNothingSaysSo rather than showing an empty version.
func TestAClaudeThatPrintsNothingSaysSo(t *testing.T) {
	fakeBin(t, map[string]string{"claude": claudeWith("exit 0")})
	if _, notice := startedWith(t, "claude"); !strings.Contains(notice, "(no version printed)") {
		t.Errorf("notice = %q", notice)
	}
}

// TestTheCheckRunsInTheAgentsEnvironment: the same scrub as the agent, so a
// key Deck keeps from the agent is kept from the check too.
func TestTheCheckRunsInTheAgentsEnvironment(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "leaked")
	fakeBin(t, map[string]string{"claude": claudeWith("echo ${ANTHROPIC_API_KEY:-clean}; exit 0")})
	if _, notice := startedWith(t, "claude"); !strings.Contains(notice, "(clean)") {
		t.Errorf("notice = %q, want the check run without the key", notice)
	}
}

// TestARelativeAgentIsNamedFromTheSessionDir: the child runs in the session's
// directory, so a relative command names a file there. Deck's own directory
// holds a different one, whose version the notice must not give.
func TestARelativeAgentIsNamedFromTheSessionDir(t *testing.T) {
	fakeBin(t, nil)
	deckDir, sessDir := t.TempDir(), t.TempDir()
	for dir, version := range map[string]string{deckDir: "1.0.0", sessDir: "9.9.9"} {
		if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := claudeWith("echo " + version + "; exit 0")
		if err := os.WriteFile(filepath.Join(dir, "b", "claude"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(deckDir)

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "s", Agent: "b/claude", Dir: sessDir})
	m := sized(New(st, "b/claude", nil), 120, 30)
	m.screen = screenSession
	t.Cleanup(func() { m.Close() })
	next, cmd := m.attach()
	m = next.(Model)
	if m.fault != nil {
		t.Fatalf("start: %v", m.fault)
	}
	after, _ := m.Update(cmd())
	notice := after.(Model).notice
	if want := "started b/claude (9.9.9) in " + sessDir; notice != want {
		t.Errorf("notice = %q, want %q: the version of the file the agent ran", notice, want)
	}
}

// TestAnOlderAnswerDoesNotCompleteANewerStart: two starts in one directory
// set identical notices, and the first start's answer belongs to the first.
func TestAnOlderAnswerDoesNotCompleteANewerStart(t *testing.T) {
	m := Model{}
	m.announceStart("/bin/claude", "claude", "/d")
	older := agentVersionMsg{started: m.notice, agentPath: "/bin/claude", dir: "/d", info: "1.0", seq: m.starts}
	m.announceStart("/bin/claude", "claude", "/d")
	m.applyVersion(older)
	if strings.Contains(m.notice, "1.0") {
		t.Errorf("the first start's answer completed the second's notice: %q", m.notice)
	}
}
