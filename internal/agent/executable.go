package agent

// Which file an agent runs: the path exec resolved, kept on the Runner, and
// the check that refuses a missing file or a directory with that reason
// rather than with the error the PTY start would give.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Path is the file the agent runs. See Runner.path.
func (r *Runner) Path() string { return r.path }

// Command is the command the agent was started with, as given. A session's
// own record says what its next start runs, which a switch can change while
// this process is still the one in the pane.
func (r *Runner) Command() string { return r.cfg.Command }

// checkCommandFile refuses a command named by path that is missing or a
// directory, with that reason. exec would refuse it too, but at fork as
// "permission denied" for a directory. The path is handed to the kernel as
// written, joined by a plain separator rather than filepath.Join, which would
// take ".." textually where the kernel takes it after a symlink — the stat then
// checks the file exec will run. A bare name was already found on PATH.
func checkCommandFile(path, dir string) error {
	if !strings.ContainsRune(path, filepath.Separator) {
		return nil
	}
	at := path
	if dir != "" && !filepath.IsAbs(path) {
		at = dir + string(filepath.Separator) + path
	}
	info, err := os.Stat(at)
	if err != nil {
		return fmt.Errorf("agent %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("agent %q is a directory", path)
	}
	return nil
}

// LookupReason says why a bare command name did not resolve on PATH, for an
// error from exec.LookPath or exec.Cmd.Err. exec refuses a file found only
// through a relative PATH entry such as ".", and that file was found:
// reporting it as "not found" sent the reader after the wrong problem.
func LookupReason(err error) string {
	if errors.Is(err, exec.ErrDot) {
		return "found only through a relative PATH entry, which is refused — put an absolute directory on PATH"
	}
	return "not found on PATH"
}
