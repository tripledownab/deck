package agent

// Which file an agent runs: the path exec resolved, kept on the Runner, and
// the check that refuses a missing file or a directory with that reason
// rather than with the error the PTY start would give.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path is the file the agent runs. See Runner.path.
func (r *Runner) Path() string { return r.path }

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
