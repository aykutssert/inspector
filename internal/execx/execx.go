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
	return ErrWithOutput(err, nil)
}

// ErrWithOutput wraps err with stderr, falling back to stdout when tools report
// structured errors there (Semgrep does this for invalid YAML/config files).
func ErrWithOutput(err error, stdout []byte) error {
	var ee *exec.ExitError
	if e, ok := err.(*exec.ExitError); ok {
		ee = e
	}
	if ee == nil {
		return err
	}
	msg := strings.TrimSpace(string(ee.Stderr))
	if msg == "" {
		msg = strings.TrimSpace(string(stdout))
	}
	if msg == "" {
		return err
	}
	if len(msg) > 2000 { // keep reports readable; the tail rarely adds signal
		msg = msg[:2000] + "…"
	}
	return fmt.Errorf("%w: %s", err, msg)
}
