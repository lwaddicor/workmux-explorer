// Package exec provides a small helper for running external commands
// with an explicit working directory, capturing stdout/stderr and the exit
// code, without ever invoking a shell.
package exec

import (
	"bytes"
	"errors"
	"os/exec"
)

// Result captures the outcome of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	// Err is non-nil only when the process could not be started at all
	// (e.g. the binary is not on PATH). A non-zero exit code is reported
	// via ExitCode instead.
	Err error
}

// OK reports whether the command ran and exited successfully.
func (r Result) OK() bool { return r.Err == nil && r.ExitCode == 0 }

// Run executes name with args, setting the working directory to dir when
// non-empty. Arguments are passed as discrete argv elements; no shell is
// involved, so user-controlled values cannot be interpreted as shell syntax.
func Run(dir string, name string, args ...string) Result {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.Err = err
			res.ExitCode = -1
		}
	}
	return res
}
