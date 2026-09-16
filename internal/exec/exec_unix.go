//go:build unix

package exec

import (
	"os/exec"
	"syscall"
)

// configureDeadlineKill makes a deadline expiry kill every descendant of the
// command, not just the direct process: the command runs in its own process
// group and the cancel hook SIGKILLs that group. An orphaned grandchild (e.g.
// a git child of workmux) would otherwise keep our I/O pipes open and outlive
// the caller's query.
func configureDeadlineKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
