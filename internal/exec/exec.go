// Package exec provides a small helper for running external commands
// with an explicit working directory, capturing stdout/stderr and the exit
// code, without ever invoking a shell.
package exec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// waitDelay bounds how long RunCtx keeps waiting on I/O after a deadline kill.
// A group SIGKILL closes every in-group pipe promptly; this only guards against
// an out-of-group orphan that somehow still holds one, so it stays short.
const waitDelay = 500 * time.Millisecond

// Result captures the outcome of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	// Err is non-nil only when the process could not be started at all
	// (e.g. the binary is not on PATH). A non-zero exit code is reported
	// via ExitCode instead; a deadline expiry is reported via TimedOut.
	Err error
	// TimedOut reports that the command was killed because its context's
	// deadline passed before it finished. The process does not outlive the
	// caller: it has been killed by the time RunCtx returns.
	TimedOut bool
}

// OK reports whether the command ran and exited successfully.
func (r Result) OK() bool { return r.Err == nil && r.ExitCode == 0 }

// Run executes name with args, setting the working directory to dir when
// non-empty, and waits for it without any deadline. Arguments are passed as
// discrete argv elements; no shell is involved, so user-controlled values
// cannot be interpreted as shell syntax.
func Run(dir string, name string, args ...string) Result {
	return RunCtx(context.Background(), dir, name, args...)
}

// RunCtx executes name with args under ctx. When ctx expires before the
// command finishes, the process is killed and the result has TimedOut set;
// Err stays nil in that case so callers can still distinguish a missing
// binary (Err) from an abandoned read (TimedOut). The working directory is
// set to dir when non-empty. Arguments are passed as discrete argv elements;
// no shell is involved, so user-controlled values cannot be interpreted as
// shell syntax.
func RunCtx(ctx context.Context, dir string, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	configureDeadlineKill(cmd)
	cmd.WaitDelay = waitDelay
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	res := Result{}
	if startErr := cmd.Start(); startErr != nil {
		res.Err = startErr
		res.ExitCode = -1
		return res
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			res.ExitCode = ee.ExitCode()
		} else if ctx.Err() == nil {
			res.Err = waitErr
			res.ExitCode = -1
		}
	}
	res.Stdout = out.String()
	res.Stderr = errb.String()
	if ctx.Err() != nil && res.Err == nil {
		res.TimedOut = true
	}
	return res
}
