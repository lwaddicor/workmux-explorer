//go:build !unix

package exec

import (
	"os/exec"
)

// configureDeadlineKill is a no-op where there are no process groups; on
// deadline expiry os/exec's default cancel already terminates the direct
// process, which is the most this platform can do.
func configureDeadlineKill(cmd *exec.Cmd) {}
