// Package focus detects the terminal application hosting a tmux session and
// brings it to the operating-system foreground. It is a best-effort, macOS-only
// concern: when the host app cannot be determined or the platform does not
// support activation, it reports a readable note in Result instead of failing,
// so the caller can still complete the tmux window switch.
package focus

import (
	"fmt"
	"path"
	"runtime"
	"strconv"
	"strings"

	"gittreemux/internal/exec"
)

// Runner executes a command without a shell and returns its captured output.
// It mirrors exec.Run's signature so tests can substitute canned output.
type Runner func(dir, name string, args ...string) exec.Result

// Result reports the outcome of an activation attempt. Activated is true only
// when the host terminal was successfully brought to the front. Note is a
// human-readable explanation, set whenever Activated is false.
type Result struct {
	App       string
	Activated bool
	Note      string
}

// maxAncestorDepth bounds the ps parent-chain walk so a malformed process
// table cannot loop forever.
const maxAncestorDepth = 64

// knownApps maps a terminal's process comm (lower-cased) to the exact macOS
// application name AppleScript can address. Unknown comms fall back to a
// capitalized form of the comm.
var knownApps = map[string]string{
	"terminal":  "Terminal",
	"term":      "Terminal",
	"iterm":     "iTerm2",
	"iterm2":    "iTerm2",
	"ghostty":   "Ghostty",
	"alacritty": "Alacritty",
	"kitty":     "kitty",
	"warp":      "Warp",
	"wezterm":   "WezTerm",
	"hyper":     "Hyper",
	"hyper.js":  "Hyper",
}

// Activator performs host-terminal detection and activation. Zero-value
// fields fall back to the real exec.Run runner and the host's GOOS.
type Activator struct {
	// Run executes the tmux/ps/osascript commands. Defaults to exec.Run.
	Run Runner
	// GOOS overrides runtime.GOOS; primarily used by tests to exercise the
	// non-darwin path.
	GOOS string
}

// New returns an Activator that shells out via exec.Run on the host platform.
func New() *Activator { return &Activator{} }

func (a *Activator) run(name string, args ...string) exec.Result {
	if a.Run != nil {
		return a.Run("", name, args...)
	}
	return exec.Run("", name, args...)
}

func (a *Activator) platform() string {
	if a.GOOS != "" {
		return a.GOOS
	}
	return runtime.GOOS
}

// ActivateSession resolves the terminal application hosting the tmux session
// and brings it to the foreground. It never reports a hard error: best-effort
// outcomes and failures are conveyed through the returned Result.
func (a *Activator) ActivateSession(session string) Result {
	if a.platform() != "darwin" {
		return Result{Activated: false, Note: "bringing a terminal to the front is not supported on this platform"}
	}
	if strings.TrimSpace(session) == "" {
		return Result{Activated: false, Note: "no tmux session was provided, so no terminal could be activated"}
	}

	res := a.run("tmux", "list-clients", "-t", session, "-F", "#{client_pid}")
	if !res.OK() {
		return Result{Activated: false, Note: "no terminal is attached to this tmux session (it appears to be detached)"}
	}
	clientPID, note, ok := firstClientPID(res.Stdout)
	if !ok {
		return Result{Activated: false, Note: note}
	}

	comm, ok := a.topLevelComm(clientPID)
	if !ok {
		return Result{Activated: false, Note: "could not identify the terminal application hosting this session"}
	}

	app := displayName(comm)
	script := "tell application \"" + escapeAppleScript(app) + "\" to activate"
	res = a.run("osascript", "-e", script)
	if !res.OK() {
		return Result{App: app, Activated: false, Note: "could not bring " + app + " to the front: " + commandDetail(res)}
	}
	return Result{App: app, Activated: true}
}

// firstClientPID returns the first valid client PID from tmux list-clients
// output. When no valid PID is present it returns ok=false and a note that
// distinguishes an empty session from a malformed one.
func firstClientPID(out string) (int, string, bool) {
	sawInvalid := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil && pid > 0 {
			return pid, "", true
		}
		sawInvalid = true
	}
	if sawInvalid {
		return 0, "could not determine the terminal (tmux reported an invalid client PID)", false
	}
	return 0, "no terminal is attached to this tmux session (it appears to be detached)", false
}

// topLevelComm walks the process parent chain from pid until it reaches the
// process whose parent is launchd (PID 1), returning that process's comm. That
// is the top-level terminal application. ok=false when the chain cannot be
// resolved.
func (a *Activator) topLevelComm(pid int) (string, bool) {
	current := pid
	for i := 0; i < maxAncestorDepth; i++ {
		res := a.run("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(current))
		if !res.OK() {
			return "", false
		}
		line := firstNonEmptyLine(res.Stdout)
		if line == "" {
			return "", false
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", false
		}
		ppid := fields[0]
		comm := strings.Join(fields[1:], " ")
		if ppid == "1" {
			return comm, true
		}
		next, err := strconv.Atoi(ppid)
		if err != nil || next <= 0 {
			return "", false
		}
		current = next
	}
	return "", false
}

func firstNonEmptyLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// displayName maps a process comm to its macOS application name, falling back
// to a capitalized comm for unknown terminals. On macOS `ps -o comm=` reports
// the full executable path for GUI apps, so the base name is used for lookup.
func displayName(comm string) string {
	base := path.Base(comm)
	if name, ok := knownApps[strings.ToLower(base)]; ok {
		return name
	}
	if base == "" {
		return ""
	}
	return strings.ToUpper(base[:1]) + base[1:]
}

// escapeAppleScript escapes backslashes and double quotes so a value can be
// embedded safely inside an AppleScript double-quoted string literal.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// commandDetail formats a failed command's message, preferring stderr over
// stdout and falling back to the exit code.
func commandDetail(res exec.Result) string {
	detail := strings.TrimSpace(res.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(res.Stdout)
	}
	if detail == "" {
		detail = fmt.Sprintf("exited with code %d", res.ExitCode)
	}
	return detail
}
