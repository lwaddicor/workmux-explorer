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

	"github.com/lwaddicor/gittreemux/internal/exec"
)

// Runner executes a command without a shell and returns its captured output.
// It mirrors exec.Run's signature so tests can substitute canned output.
type Runner func(dir, name string, args ...string) exec.Result

// Result reports the outcome of an activation attempt. Activated is true only
// when the host terminal was successfully brought to the front. Note is a
// human-readable explanation, set whenever Activated is false or the terminal
// came forward without the target window being surfaced.
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
// and brings it to the foreground. paneID is the tmux pane id of the window to
// surface (with or without its leading "%"); it may be empty, in which case
// only the application is raised. It never reports a hard error: best-effort
// outcomes and failures are conveyed through the returned Result.
func (a *Activator) ActivateSession(session, paneID string) Result {
	if a.platform() != "darwin" {
		return Result{Activated: false, Note: "bringing a terminal to the front is not supported on this platform"}
	}
	if strings.TrimSpace(session) == "" {
		return Result{Activated: false, Note: "no tmux session was provided, so no terminal could be activated"}
	}

	res := a.run("tmux", "list-clients", "-t", session, "-F", "#{client_pid}\t#{client_tty}")
	if !res.OK() {
		return Result{Activated: false, Note: "no terminal is attached to this tmux session (it appears to be detached)"}
	}
	clientPID, tty, note, ok := firstClient(res.Stdout)
	if !ok {
		return Result{Activated: false, Note: note}
	}

	comm, ok := a.topLevelComm(clientPID)
	if !ok {
		return Result{Activated: false, Note: "could not identify the terminal application hosting this session"}
	}

	app := displayName(comm)
	script := activationScript(app, tty, strings.TrimPrefix(strings.TrimSpace(paneID), "%"))
	res = a.run("osascript", "-e", script)
	if !res.OK() {
		return Result{App: app, Activated: false, Note: "could not bring " + app + " to the front: " + commandDetail(res)}
	}
	if strings.TrimSpace(res.Stdout) == outcomeApp {
		return Result{App: app, Activated: true, Note: "brought " + app + " to the front, but could not find the tab showing this window"}
	}
	return Result{App: app, Activated: true}
}

// firstClient returns the first valid client PID and its tty from
// `tmux list-clients` output formatted as "<pid>\t<tty>". When no valid PID
// is present it returns ok=false and a note that distinguishes an empty
// session from a malformed one. A line without a tty column yields an empty
// tty, which callers treat as "terminal found but location unknown".
func firstClient(out string) (pid int, tty string, note string, ok bool) {
	sawInvalid := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || p <= 0 {
			sawInvalid = true
			continue
		}
		if len(parts) == 2 {
			tty = strings.TrimSpace(parts[1])
		}
		return p, tty, "", true
	}
	if sawInvalid {
		return 0, "", "could not determine the terminal (tmux reported an invalid client PID)", false
	}
	return 0, "", "no terminal is attached to this tmux session (it appears to be detached)", false
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

// outcomeApp is the token the iTerm2 script prints when it could not locate the
// tab for the target window and raised the whole application instead.
const outcomeApp = "app"

// activationScript returns the AppleScript that surfaces the terminal window
// running the target tmux window.
//
// iTerm2 needs more than an application-level activate, which would only
// surface whichever iTerm2 window was last active. It is matched in two passes.
// The first compares paneID against each session's session.tmuxWindowPane
// variable: under iTerm2's tmux integration (tmux -CC) every tmux window is a
// native iTerm2 tab that reports no tty, so the pane id is the only handle onto
// it. The second compares the client tty, which covers a plain tmux client
// running inside an ordinary iTerm2 session; the tmux gateway session is
// excluded there because it carries the client tty while displaying none of the
// session's windows. Every other terminal, and iTerm2 with neither handle
// known, is sent a plain application-level activate.
func activationScript(app, tty, paneID string) string {
	if app != "iTerm2" || (paneID == "" && tty == "") {
		return "tell application \"" + escapeAppleScript(app) + "\" to activate"
	}
	var b strings.Builder
	b.WriteString("tell application \"iTerm2\"\n")
	if paneID != "" {
		b.WriteString(itermSelectPass(`(variable s named "session.tmuxWindowPane") is "`+escapeAppleScript(paneID)+`"`, "pane"))
	}
	if tty != "" {
		b.WriteString(itermSelectPass(`(tty of s) is "`+escapeAppleScript(tty)+`" and (variable s named "session.tmuxRole") is not "gateway"`, "tty"))
	}
	b.WriteString("\tactivate\n\treturn \"" + outcomeApp + "\"\nend tell")
	return b.String()
}

// itermSelectPass walks every iTerm2 session and, for the first one matching
// cond, selects that session, its tab and its window, activates iTerm2 and
// returns token. Each test is guarded because sessions of terminals without
// tmux integration do not answer the tmux variables.
func itermSelectPass(cond, token string) string {
	return `	repeat with w in (windows)
		repeat with t in (tabs of w)
			repeat with s in (sessions of t)
				try
					if ` + cond + ` then
						select s
						select t
						select w
						activate
						return "` + token + `"
					end if
				end try
			end repeat
		end repeat
	end repeat
`
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
