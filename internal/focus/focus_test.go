package focus

import (
	"strings"
	"testing"

	"gittreemux/internal/exec"
)

// fakeRunner returns canned output for tmux/ps/osascript and records every
// invocation so tests can assert on the exact argv (in particular the
// osascript script).
type fakeRunner struct {
	calls        []string
	tmuxOut      string
	tmuxExit     int
	ps           map[string]string // pid -> "ppid<TAB>comm"
	osascriptOK  bool
	osascriptErr string
}

func (f *fakeRunner) run(dir, name string, args ...string) exec.Result {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	switch name {
	case "tmux":
		return exec.Result{Stdout: f.tmuxOut, ExitCode: f.tmuxExit}
	case "ps":
		pid := args[len(args)-1]
		if v, ok := f.ps[pid]; ok {
			return exec.Result{Stdout: v + "\n", ExitCode: 0}
		}
		return exec.Result{Stderr: "ps: " + pid + ": No such process", ExitCode: 1}
	case "osascript":
		if f.osascriptOK {
			return exec.Result{ExitCode: 0}
		}
		return exec.Result{Stderr: f.osascriptErr, ExitCode: 1}
	}
	return exec.Result{}
}

func (f *fakeRunner) called(name string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, name+" ") {
			return true
		}
	}
	return false
}

func (f *fakeRunner) callWith(needle string) bool {
	for _, c := range f.calls {
		if strings.Contains(c, needle) {
			return true
		}
	}
	return false
}

// chain builds a ps table where pid -> ppid,comm forms a chain that terminates
// at the app whose parent is launchd (pid 1).
func TestActivateSessionDetectsAndActivatesTerminal(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\n",
		// Real macOS ps output is right-justified and space-separated, not
		// tab-separated, so the parser must be whitespace-tolerant.
		ps: map[string]string{
			"1000": "     2000 tmux",
			"2000": "     3000 zsh",
			"3000": "        1 Terminal",
		},
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0")
	if !res.Activated {
		t.Fatalf("expected activation, got note %q", res.Note)
	}
	if res.App != "Terminal" {
		t.Errorf("expected app Terminal, got %q", res.App)
	}
	if !fr.callWith(`tell application "Terminal" to activate`) {
		t.Errorf("expected osascript to activate Terminal; calls: %v", fr.calls)
	}
}

func TestActivateSessionMapsKittyComm(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "42\n",
		ps: map[string]string{
			"42": "1\tkitty",
		},
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("3")
	if res.App != "kitty" {
		t.Errorf("expected app kitty, got %q", res.App)
	}
	if !fr.callWith(`tell application "kitty" to activate`) {
		t.Errorf("expected osascript to activate kitty; calls: %v", fr.calls)
	}
}

func TestActivateSessionFullAppPath(t *testing.T) {
	// macOS `ps -o comm=` reports the full executable path for GUI apps; the
	// base name must be mapped to the app bundle name for AppleScript.
	fr := &fakeRunner{
		tmuxOut: "1000\n",
		ps: map[string]string{
			"1000": "     2000 tmux",
			"2000": "        1 /Applications/iTerm.app/Contents/MacOS/iTerm2",
		},
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0")
	if res.App != "iTerm2" {
		t.Errorf("expected app iTerm2, got %q", res.App)
	}
	if !fr.callWith(`tell application "iTerm2" to activate`) {
		t.Errorf("expected osascript to activate iTerm2; calls: %v", fr.calls)
	}
}

func TestActivateSessionDetached(t *testing.T) {
	fr := &fakeRunner{tmuxOut: ""} // no attached clients
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0")
	if res.Activated {
		t.Fatalf("expected no activation for a detached session")
	}
	if res.Note == "" {
		t.Errorf("expected a note explaining the missing terminal")
	}
	if fr.called("osascript") {
		t.Errorf("must not attempt activation when no client is attached")
	}
}

func TestActivateSessionNonDarwin(t *testing.T) {
	fr := &fakeRunner{tmuxOut: "1000\n"}
	a := &Activator{Run: fr.run, GOOS: "linux"}

	res := a.ActivateSession("0")
	if res.Activated {
		t.Fatalf("expected no activation on a non-darwin platform")
	}
	if !strings.Contains(res.Note, "not supported") {
		t.Errorf("expected a not-supported note, got %q", res.Note)
	}
	if fr.called("tmux") || fr.called("osascript") {
		t.Errorf("must not run tmux or osascript on a non-darwin platform")
	}
}

func TestActivateSessionMalformedPID(t *testing.T) {
	fr := &fakeRunner{tmuxOut: "not-a-pid\n"}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0")
	if res.Activated {
		t.Fatalf("expected no activation for a malformed client PID")
	}
	if res.Note == "" {
		t.Errorf("expected a note for a malformed PID")
	}
	if fr.called("ps") || fr.called("osascript") {
		t.Errorf("must not run ps or osascript for a malformed PID")
	}
}

func TestActivateSessionUnknownAppFallback(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "7\n",
		ps: map[string]string{
			"7": "1\tmyterm",
		},
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("1")
	if res.App != "Myterm" {
		t.Errorf("expected capitalized fallback app Myterm, got %q", res.App)
	}
	if !fr.callWith(`tell application "Myterm" to activate`) {
		t.Errorf("expected osascript to activate Myterm; calls: %v", fr.calls)
	}
}

func TestActivateSessionEscapesAppQuotes(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "8\n",
		ps: map[string]string{
			"8": "1\tWe\"ird",
		},
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("2")
	if !fr.callWith(`tell application "We\"ird" to activate`) {
		t.Errorf("expected the app name to be escaped in the script; calls: %v", fr.calls)
	}
	if !res.Activated {
		t.Errorf("expected activation to succeed, note: %q", res.Note)
	}
}

func TestActivateSessionOsascriptFailure(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\n",
		ps: map[string]string{
			"1000": "1\tTerminal",
		},
		osascriptOK:  false,
		osascriptErr: "1:3: execution error: not authorized",
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0")
	if res.Activated {
		t.Fatalf("expected no activation when osascript fails")
	}
	if !strings.Contains(res.Note, "could not bring Terminal to the front") {
		t.Errorf("expected a descriptive note, got %q", res.Note)
	}
}
