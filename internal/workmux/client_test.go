package workmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleList is a captured `workmux list --json` payload (workmux 0.1.234),
// trimmed to a main worktree and two linked worktrees.
const sampleList = `[
  {"handle":"unity-player-services","branch":"main","path":"/opt/UnitySrc/repos/unity-player-services","is_main":true,"mode":"window","has_uncommitted_changes":true,"is_open":false,"created_at":1784283581},
  {"handle":"feat-enhance-matchmaker-logs","branch":"feat/MTT-15590/matchmaker-customer-log-attributes","path":"/opt/UnitySrc/repos/unity-player-services__worktrees/feat-enhance-matchmaker-logs","is_main":false,"mode":"window","has_uncommitted_changes":false,"is_open":true,"created_at":1786447457},
  {"handle":"untracked+fix+relay-connection-keepalive","branch":"untracked/fix/relay-connection-keepalive","path":"/opt/UnitySrc/repos/unity-player-services/.claude/worktrees/untracked+fix+relay-connection-keepalive","is_main":false,"mode":"window","has_uncommitted_changes":false,"is_open":false,"created_at":1785521185}
]`

func TestParseList(t *testing.T) {
	got, err := parseList([]byte(sampleList))
	if err != nil {
		t.Fatalf("parseList returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(got))
	}

	main := got[0]
	if !main.IsMain {
		t.Errorf("worktree 0: expected is_main true")
	}
	if main.Handle != "unity-player-services" {
		t.Errorf("worktree 0: unexpected handle %q", main.Handle)
	}
	if main.Branch != "main" {
		t.Errorf("worktree 0: unexpected branch %q", main.Branch)
	}
	if !main.HasUncommittedChanges {
		t.Errorf("worktree 0: expected has_uncommitted_changes true")
	}
	if main.CreatedAt != 1784283581 {
		t.Errorf("worktree 0: unexpected created_at %d", main.CreatedAt)
	}

	open := got[1]
	if !open.IsOpen {
		t.Errorf("worktree 1: expected is_open true")
	}
	if open.Agent != nil {
		t.Errorf("worktree 1: list should not populate Agent (that is status's job)")
	}

	// Handles with plus signs (encoded '/') must survive verbatim.
	if got[2].Handle != "untracked+fix+relay-connection-keepalive" {
		t.Errorf("worktree 2: unexpected handle %q", got[2].Handle)
	}
}

func TestParseListInvalid(t *testing.T) {
	if _, err := parseList([]byte(`not json`)); err == nil {
		t.Fatalf("expected an error for non-JSON input")
	}
}

// sampleStatus is a captured `workmux status --json --git` payload. Note the
// second entry has a null agent_kind, which must not error.
const sampleStatus = `[
  {
    "worktree": "feat-enhance-matchmaker-logs",
    "branch": "feat/MTT-15590/matchmaker-customer-log-attributes",
    "status": "done",
    "elapsed_secs": 186840,
    "title": "Improve matchmaking logs identifiers and structure",
    "agent_kind": "claude",
    "pane_id": "%8",
    "workdir": "/opt/UnitySrc/repos/unity-player-services__worktrees/feat-enhance-matchmaker-logs",
    "session": "0",
    "window_name": "wm-feat-enhance-matchmaker-logs",
    "updated_ts": 1786635842,
    "git": {"has_staged": false, "has_unstaged": false, "has_unmerged_commits": true}
  },
  {
    "worktree": "investigation-cloud-code-timeouts",
    "branch": "investigation/cloud-code-timeouts",
    "status": "working",
    "elapsed_secs": 18826,
    "title": "Claude Code",
    "agent_kind": null,
    "git": {"has_staged": false, "has_unstaged": true, "has_unmerged_commits": true}
  }
]`

func TestParseStatus(t *testing.T) {
	got, err := parseStatus([]byte(sampleStatus))
	if err != nil {
		t.Fatalf("parseStatus returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(got))
	}

	s0 := got[0]
	if s0.Worktree != "feat-enhance-matchmaker-logs" {
		t.Errorf("status 0: unexpected worktree %q", s0.Worktree)
	}
	if s0.Status != StatusDone {
		t.Errorf("status 0: expected status done, got %q", s0.Status)
	}
	if s0.ElapsedSecs != 186840 {
		t.Errorf("status 0: unexpected elapsed_secs %d", s0.ElapsedSecs)
	}
	if s0.AgentKind == nil || *s0.AgentKind != "claude" {
		t.Errorf("status 0: expected agent_kind claude, got %v", s0.AgentKind)
	}
	if !s0.Git.HasUnmergedCommits {
		t.Errorf("status 0: expected has_unmerged_commits true")
	}

	// Null agent_kind must parse to nil without error.
	if got[1].AgentKind != nil {
		t.Errorf("status 1: expected nil agent_kind for null JSON, got %v", *got[1].AgentKind)
	}
	if !got[1].Git.HasUnstaged {
		t.Errorf("status 1: expected has_unstaged true")
	}
}

func TestJoin(t *testing.T) {
	wts, _ := parseList([]byte(sampleList))
	sts, _ := parseStatus([]byte(sampleStatus))
	joined := Join(wts, sts)

	byHandle := make(map[string]*Worktree, len(joined))
	for i := range joined {
		byHandle[joined[i].Handle] = &joined[i]
	}

	if a := byHandle["feat-enhance-matchmaker-logs"].Agent; a == nil || a.Status != StatusDone {
		t.Errorf("feat-enhance-matchmaker-logs: expected a done agent, got %v", byHandle["feat-enhance-matchmaker-logs"].Agent)
	}
	if a := byHandle["unity-player-services"].Agent; a != nil {
		t.Errorf("unity-player-services: expected no agent, got %v", a)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{
		"feat-enhance-matchmaker-logs",
		"untracked+fix+relay-connection-keepalive",
		"agent-a1ff69ca681ce4be7",
		"unity-player-services",
		"single",
		"has.dots_and-dashes+plus",
	}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) unexpected error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"a/b",          // slash
		"a b",          // space
		"rm -rf /",     // shell-ish
		";id",          // metachar
		"$(cmd)",       // metachars
		"a..b/../../x", // path traversal
		"中文",           // non-ascii
		"foo\nbar",     // newline
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) expected an error, got nil", name)
		}
	}
}

// setupFakeWorkmux installs a stub `workmux` executable that records its
// working directory and argv to a log, so the action wrappers can be verified
// without invoking the real binary (and without any side effects).
func setupFakeWorkmux(t *testing.T) (binPath, projDir string) {
	t.Helper()
	dir := t.TempDir()
	binPath = filepath.Join(dir, "fake-workmux")
	logPath := filepath.Join(dir, "args.log")
	script := `#!/bin/sh
{
  printf 'INV\n'
  printf 'CWD %s\n' "$(pwd)"
  for a in "$@"; do printf 'ARG %s\n' "$a"; done
} >> "$GT_FAKE_LOG"
exit 0
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("GT_FAKE_LOG", logPath)
	return binPath, dir
}

func TestActionWrappersEmitExpectedArgv(t *testing.T) {
	binPath, projDir := setupFakeWorkmux(t)
	logPath := filepath.Join(filepath.Dir(binPath), "args.log")
	c := &Client{Bin: binPath}

	if err := c.Open(projDir, "my-handle"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := c.Close(projDir, "my-handle"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := c.Remove(projDir, "my-handle", true); err != nil {
		t.Fatalf("remove (force): %v", err)
	}
	if err := c.Remove(projDir, "my-handle", false); err != nil {
		t.Fatalf("remove (no force): %v", err)
	}
	if err := c.Send(projDir, "my-handle", "please continue"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// An invalid name must be rejected before any binary invocation.
	if err := c.Open(projDir, "bad/name"); err == nil {
		t.Fatalf("expected Open to reject invalid worktree name")
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")

	want := []string{
		"INV", "CWD " + projDir, "ARG open", "ARG my-handle",
		"INV", "CWD " + projDir, "ARG close", "ARG my-handle",
		"INV", "CWD " + projDir, "ARG remove", "ARG -f", "ARG my-handle",
		"INV", "CWD " + projDir, "ARG remove", "ARG my-handle",
		"INV", "CWD " + projDir, "ARG send", "ARG my-handle", "ARG please continue",
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d log lines, got %d:\n%s", len(want), len(lines), string(b))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("log line %d: got %q, want %q", i, lines[i], want[i])
		}
	}
}
