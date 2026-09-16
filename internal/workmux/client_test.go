package workmux

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// sampleStatusWrapped is a captured `workmux status --json --git` payload from
// workmux 0.1.246+, which wraps the agent list in a top-level object.
const sampleStatusWrapped = `{
  "context": {"backend": "tmux", "instance": "/private/tmp/tmux-501/default"},
  "scope": {"repository": "/opt/UnitySrc/repos/gittreemux", "targets": []},
  "state_files_total": 3,
  "reconciled_agent_count": 2,
  "agents": [
    {
      "worktree": "feat-update-instantly-after-remove",
      "branch": "feat/update-instantly-after-remove",
      "status": "working",
      "elapsed_secs": 136,
      "title": "Fix workmux AgentStatus JSON unmarshal",
      "pane_id": "%10",
      "workdir": "/opt/UnitySrc/repos/gittreemux__worktrees/feat-update-instantly-after-remove",
      "agent_kind": "opencode",
      "session": "0",
      "window_name": "wm-feat-update-instantly-after-remove",
      "updated_ts": 1787746096,
      "git": {"has_staged": false, "has_unstaged": true, "has_unmerged_commits": false}
    },
    {
      "worktree": "chore-cleanup-rubbish",
      "branch": "chore/cleanup-rubbish",
      "status": "done",
      "elapsed_secs": 577235,
      "title": "OC | Git hooks workmux-status file inclusion",
      "pane_id": "%24",
      "workdir": "/opt/UnitySrc/repos/gittreemux__worktrees/chore-cleanup-rubbish",
      "agent_kind": null,
      "session": "0",
      "window_name": "wm-chore-cleanup-rubbish",
      "updated_ts": 1787168997,
      "git": {"has_staged": false, "has_unstaged": false, "has_unmerged_commits": true}
    }
  ],
  "target_errors": []
}`

func TestParseStatusWrapped(t *testing.T) {
	got, err := parseStatus([]byte(sampleStatusWrapped))
	if err != nil {
		t.Fatalf("parseStatus returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(got))
	}
	s0 := got[0]
	if s0.Worktree != "feat-update-instantly-after-remove" {
		t.Errorf("status 0: unexpected worktree %q", s0.Worktree)
	}
	if s0.Status != StatusWorking {
		t.Errorf("status 0: expected status working, got %q", s0.Status)
	}
	if !s0.Git.HasUnstaged {
		t.Errorf("status 0: expected has_unstaged true")
	}
	if got[1].AgentKind != nil {
		t.Errorf("status 1: expected nil agent_kind for null JSON, got %v", *got[1].AgentKind)
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

// TestTimeoutKillsSlowCommand verifies that a wedged workmux invocation is
// killed at the client timeout and surfaced as a timed-out error (not
// "not installed"), without hanging the caller.
func TestTimeoutKillsSlowCommand(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-workmux")
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	c := &Client{Bin: binPath, Timeout: 200 * time.Millisecond}

	start := time.Now()
	if _, err := c.List(dir); err == nil {
		t.Fatal("expected an error for a command that exceeds the timeout")
	} else if errors.Is(err, ErrNotInstalled) {
		t.Fatalf("a timed-out read must not be reported as %q", ErrNotInstalled)
	} else if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected a timed-out error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Errorf("List did not return promptly; the sleep was not killed (waited %s)", elapsed)
	}

	// The mutating path (runOK) maps timeouts identically.
	start = time.Now()
	err := c.Remove(dir, "h", true)
	if err == nil || !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected Remove to report a timeout, got: %v", err)
	} else if errors.Is(err, ErrNotInstalled) {
		t.Fatalf("a timed-out action must not be reported as %q", ErrNotInstalled)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Errorf("Remove did not return promptly; the sleep was not killed (waited %s)", elapsed)
	}

	// A zero timeout keeps the legacy unbounded behavior and must not report a
	// timeout for a command that completes in time.
	fast := filepath.Join(dir, "fast-workmux")
	if err := os.WriteFile(fast, []byte("#!/bin/sh\necho 0.1.0\n"), 0o755); err != nil {
		t.Fatalf("write fast binary: %v", err)
	}
	c2 := &Client{Bin: fast} // Timeout zero => no deadline
	if v, err := c2.Version(); err != nil || !strings.Contains(v, "0.1.0") {
		t.Errorf("zero-timeout Version should succeed, got %q, %v", v, err)
	}
}
