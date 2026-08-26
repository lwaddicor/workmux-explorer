package focus

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lwaddicor/workmux-explorer/internal/exec"
)

// fakeRunner returns canned output for tmux/ps/osascript and records every
// invocation so tests can assert on the exact argv (in particular the
// osascript script). ps is served as one `ps -eo ppid=,pid=,comm=` table.
type fakeRunner struct {
	calls        []string
	tmuxOut      string
	tmuxExit     int
	psTable      string // full output of `ps -eo ppid=,pid=,comm=`
	osascriptOK  bool
	osascriptOut string
	osascriptErr string
}

func (f *fakeRunner) run(dir, name string, args ...string) exec.Result {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	switch name {
	case "tmux":
		return exec.Result{Stdout: f.tmuxOut, ExitCode: f.tmuxExit}
	case "ps":
		return exec.Result{Stdout: f.psTable + "\n", ExitCode: 0}
	case "osascript":
		if f.osascriptOK {
			return exec.Result{Stdout: f.osascriptOut, ExitCode: 0}
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

// psRow formats one line of `ps -eo ppid=,pid=,comm=` the way macOS prints it:
// right-justified numbers and whitespace before the comm (which may be a full
// executable path).
func psRow(ppid, pid int, comm string) string {
	return fmt.Sprintf("%6d%6d %s", ppid, pid, comm)
}

func psTable(rows ...string) string { return strings.Join(rows, "\n") }

// chain builds a ps table where pid -> ppid,comm forms a chain that terminates
// at the app whose parent is launchd (pid 1).
func TestActivateSessionDetectsAndActivatesTerminal(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\n",
		// Real macOS ps output is right-justified and space-separated, not
		// tab-separated, so the parser must be whitespace-tolerant.
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(3000, 2000, "zsh"),
			psRow(1, 3000, "Terminal"),
		),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
	if !res.Activated {
		t.Fatalf("expected activation, got note %q", res.Note)
	}
	if res.App != "Terminal" {
		t.Errorf("expected app Terminal, got %q", res.App)
	}
	if !fr.callWith(`tell application "Terminal" to activate`) {
		t.Errorf("expected osascript to activate Terminal; calls: %v", fr.calls)
	}
	n := 0
	for _, c := range fr.calls {
		if strings.HasPrefix(c, "ps ") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the parent-chain walk must read one ps table snapshot, got %d ps calls", n)
	}
}

func TestActivateSessionMapsKittyComm(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut:     "42\n",
		psTable:     psTable(psRow(1, 42, "kitty")),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("3", "")
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
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(1, 2000, "/Applications/iTerm.app/Contents/MacOS/iTerm2"),
		),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
	if res.App != "iTerm2" {
		t.Errorf("expected app iTerm2, got %q", res.App)
	}
	if !fr.callWith(`tell application "iTerm2" to activate`) {
		t.Errorf("expected osascript to activate iTerm2; calls: %v", fr.calls)
	}
}

func TestActivateSessionSelectsITermWindowByTty(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\t/dev/ttys001\n",
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(1, 2000, "/Applications/iTerm.app/Contents/MacOS/iTerm2"),
		),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
	if !res.Activated {
		t.Fatalf("expected activation, got note %q", res.Note)
	}
	if res.App != "iTerm2" {
		t.Errorf("expected app iTerm2, got %q", res.App)
	}
	if !fr.callWith(`(tty of s) is "/dev/ttys001"`) {
		t.Errorf("expected the iTerm script to target the client tty; calls: %v", fr.calls)
	}
	if !fr.callWith("select w") || !fr.callWith("select t") || !fr.callWith("select s") {
		t.Errorf("expected the iTerm script to select the matching window, tab and pane; calls: %v", fr.calls)
	}
	if fr.callWith(`tell application "iTerm2" to activate`) {
		t.Errorf("should not fall back to a bare activate when a tty is known; calls: %v", fr.calls)
	}
}

func TestActivateSessionSelectsITermTabByPaneID(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\t/dev/ttys001\n",
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(1, 2000, "/Applications/iTerm.app/Contents/MacOS/iTerm2"),
		),
		osascriptOK:  true,
		osascriptOut: "pane\n",
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "%36")
	if !res.Activated {
		t.Fatalf("expected activation, got note %q", res.Note)
	}
	if res.Note != "" {
		t.Errorf("expected no note when the tab was found, got %q", res.Note)
	}
	if !fr.callWith(`(variable s named "session.tmuxWindowPane") is "36"`) {
		t.Errorf("expected the pane id to be matched without its leading %%; calls: %v", fr.calls)
	}
	if !fr.callWith(`(variable s named "session.tmuxRole") is not "gateway"`) {
		t.Errorf("expected the tty pass to exclude the tmux gateway session; calls: %v", fr.calls)
	}
	script := fr.calls[len(fr.calls)-1]
	if strings.Index(script, "session.tmuxWindowPane") > strings.Index(script, "(tty of s)") {
		t.Errorf("expected the pane id pass to run before the tty pass; script: %s", script)
	}
}

func TestActivateSessionNotesUnfoundITermTab(t *testing.T) {
	// Under iTerm2's tmux integration a window created after the control-mode
	// client attached may have no tab at all; the app still comes forward.
	fr := &fakeRunner{
		tmuxOut: "1000\t/dev/ttys001\n",
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(1, 2000, "/Applications/iTerm.app/Contents/MacOS/iTerm2"),
		),
		osascriptOK:  true,
		osascriptOut: "app\n",
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "%99")
	if !res.Activated {
		t.Fatalf("expected the app to still be activated, got note %q", res.Note)
	}
	if !strings.Contains(res.Note, "could not find the tab") {
		t.Errorf("expected a note about the missing tab, got %q", res.Note)
	}
}

func TestActivateSessionIgnoresPaneIDForOtherTerminals(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut: "1000\t/dev/ttys001\n",
		psTable: psTable(
			psRow(2000, 1000, "tmux"),
			psRow(1, 2000, "Ghostty"),
		),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "%36")
	if !res.Activated {
		t.Fatalf("expected activation, got note %q", res.Note)
	}
	if !fr.callWith(`tell application "Ghostty" to activate`) {
		t.Errorf("expected a plain activate for a terminal without tab scripting; calls: %v", fr.calls)
	}
}

func TestActivateSessionDetached(t *testing.T) {
	fr := &fakeRunner{tmuxOut: ""} // no attached clients
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
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

	res := a.ActivateSession("0", "")
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

	res := a.ActivateSession("0", "")
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
		tmuxOut:     "7\n",
		psTable:     psTable(psRow(1, 7, "myterm")),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("1", "")
	if res.App != "Myterm" {
		t.Errorf("expected capitalized fallback app Myterm, got %q", res.App)
	}
	if !fr.callWith(`tell application "Myterm" to activate`) {
		t.Errorf("expected osascript to activate Myterm; calls: %v", fr.calls)
	}
}

func TestActivateSessionEscapesAppQuotes(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut:     "8\n",
		psTable:     psTable(psRow(1, 8, `We"ird`)),
		osascriptOK: true,
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("2", "")
	if !fr.callWith(`tell application "We\"ird" to activate`) {
		t.Errorf("expected the app name to be escaped in the script; calls: %v", fr.calls)
	}
	if !res.Activated {
		t.Errorf("expected activation to succeed, note: %q", res.Note)
	}
}

func TestActivateSessionOsascriptFailure(t *testing.T) {
	fr := &fakeRunner{
		tmuxOut:      "1000\n",
		psTable:      psTable(psRow(1, 1000, "Terminal")),
		osascriptOK:  false,
		osascriptErr: "1:3: execution error: not authorized",
	}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
	if res.Activated {
		t.Fatalf("expected no activation when osascript fails")
	}
	if !strings.Contains(res.Note, "could not bring Terminal to the front") {
		t.Errorf("expected a descriptive note, got %q", res.Note)
	}
}

// TestActivateSessionDepthBound verifies that a parent chain which does not
// reach launchd within maxAncestorDepth cannot loop forever: the walk gives up
// and reports that no terminal could be identified.
func TestActivateSessionDepthBound(t *testing.T) {
	n := maxAncestorDepth + 8
	rows := make([]string, n)
	for i := 0; i < n; i++ {
		pid := 100 + i
		ppid := 100 + (i+1)%n // a cycle: no process is a child of launchd
		rows[i] = psRow(ppid, pid, "sh")
	}
	fr := &fakeRunner{tmuxOut: fmt.Sprintf("%d\n", 100+n/2), psTable: psTable(rows...)}
	a := &Activator{Run: fr.run, GOOS: "darwin"}

	res := a.ActivateSession("0", "")
	if res.Activated {
		t.Fatalf("expected no activation for an unresolvable parent chain")
	}
	if !strings.Contains(res.Note, "could not identify the terminal application") {
		t.Errorf("expected the identification note, got %q", res.Note)
	}
	if fr.called("osascript") {
		t.Errorf("must not attempt activation when the terminal cannot be identified")
	}
}
