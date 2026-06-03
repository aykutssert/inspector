// Package execx normalizes errors from exec.Command runs. os/exec captures a
// failed process's stderr into (*exec.ExitError).Stderr when Output() is used,
// but its Error() string is only "exit status N". Err surfaces the stderr so a
// scanner failure reports *why* it broke, not just that it did.
package execx

import (
	"fmt"
	"os/exec"
	"strings"
)

// Err wraps err with the process stderr when available. Non-exec errors pass
// through unchanged.
func Err(err error) error {
	var ee *exec.ExitError
	if e, ok := err.(*exec.ExitError); ok {
		ee = e
	}
	if ee == nil || len(ee.Stderr) == 0 {
		return err
	}
	msg := strings.TrimSpace(string(ee.Stderr))
	if len(msg) > 2000 { // keep reports readable; the tail rarely adds signal
		msg = msg[:2000] + "…"
	}
	return fmt.Errorf("%w: %s", err, msg)
}
