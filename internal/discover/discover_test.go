package discover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lwaddicor/workmux-explorer/internal/workmux"
)

// The discovery probes shell out to `tmux` and `git` by name, so the tests
// shadow PATH with stub binaries: a tmux that reports no server (exit 1) and a
// git whose empty successful output makes every probed directory its own root.
// The workmux client points at an absolute-path stub, independent of PATH.

func writeStub(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

func newStubbed(t *testing.T, ttl time.Duration, failMode string) (*Discoverer, string, string) {
	t.Helper()
	bin := t.TempDir()
	writeStub(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 1\n")
	writeStub(t, filepath.Join(bin, "git"), "#!/bin/sh\nexit 0\n")

	logPath := filepath.Join(t.TempDir(), "calls.log")
	stub := `#!/bin/sh
{ printf 'CALL %s\n' "$1"; } >> "$GT_DISC_LOG"
case "$GT_DISC_FAIL-" in
  list-)   if [ "$1" = "list" ]; then echo boom-list >&2; exit 7; fi ;;
  status-) if [ "$1" = "status" ]; then echo boom-status >&2; exit 8; fi ;;
esac
case "$1" in
  list)      printf '%s' '[{"handle":"h1","branch":"b/h1","path":"/root/h1","is_main":true,"has_uncommitted_changes":false,"is_open":false,"created_at":1}]' ;;
  status)    printf '%s' '[]' ;;
  --version) echo 0.9-fake ;;
esac
exit 0
`
	writeStub(t, filepath.Join(bin, "wm"), stub)

	t.Setenv("PATH", bin)
	t.Setenv("GT_DISC_LOG", logPath)
	if failMode != "" {
		t.Setenv("GT_DISC_FAIL", failMode)
	}

	root := t.TempDir()
	d := New(Options{StartDir: root, Workmux: &workmux.Client{Bin: filepath.Join(bin, "wm")}, CacheTTL: ttl})
	return d, logPath, root
}

func callCount(t *testing.T, logPath, sub string) int {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read call log: %v", err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if line == "CALL "+sub {
			n++
		}
	}
	return n
}

func base(t *testing.T, root string) string {
	t.Helper()
	return filepath.Base(root)
}

// TestProjectFreshCacheAvoidsReread verifies the scoped read: an uncached
// address runs discovery once and reads the project; a fresh cached one is
// returned as-is without any further workmux call.
func TestProjectFreshCacheAvoidsReread(t *testing.T) {
	d, logPath, root := newStubbed(t, time.Hour, "")

	p1, err := d.Project(context.Background(), base(t, root))
	if err != nil {
		t.Fatalf("first Project: %v", err)
	}
	if p1.Name != filepath.Base(root) || p1.Root != root {
		t.Errorf("unexpected project identity: %+v", p1)
	}
	if len(p1.Worktrees) != 1 || p1.Worktrees[0].Handle != "h1" {
		t.Fatalf("expected one worktree h1, got %+v", p1.Worktrees)
	}
	if n := callCount(t, logPath, "list"); n != 1 {
		t.Errorf("expected exactly one list call after first read, got %d", n)
	}

	p2, err := d.Project(context.Background(), root) // addressed by full path
	if err != nil {
		t.Fatalf("second Project: %v", err)
	}
	if p2.Root != p1.Root || len(p2.Worktrees) != 1 {
		t.Errorf("cached lookup returned a different record: %+v", p2)
	}
	if n := callCount(t, logPath, "list"); n != 1 {
		t.Errorf("fresh cache hit must not re-read (list calls = %d)", n)
	}
	if n := callCount(t, logPath, "status"); n != 1 {
		t.Errorf("fresh cache hit must not re-read (status calls = %d)", n)
	}
}

// TestProjectStaleCacheRereadsOnce verifies a stale scoped entry is re-read —
// exactly once — and the fresh result is cached again.
func TestProjectStaleCacheRereadsOnce(t *testing.T) {
	d, logPath, root := newStubbed(t, time.Millisecond, "")

	if _, err := d.Project(context.Background(), base(t, root)); err != nil {
		t.Fatalf("first Project: %v", err)
	}
	time.Sleep(30 * time.Millisecond) // age the entry past its 1ms TTL

	if _, err := d.Project(context.Background(), base(t, root)); err != nil {
		t.Fatalf("second Project: %v", err)
	}
	if n := callCount(t, logPath, "list"); n != 2 {
		t.Errorf("stale entry should be re-read once (list calls = %d)", n)
	}

	if _, err := d.Project(context.Background(), base(t, root)); err != nil {
		t.Fatalf("third Project: %v", err)
	}
	if n := callCount(t, logPath, "list"); n != 2 {
		t.Errorf("the re-read result should be cached (list calls = %d)", n)
	}
}

// TestInvalidateForcesReread verifies a post-mutation Invalidate drops the
// cached record: the next scoped read re-reads the root even though the entry
// is still fresh under the TTL.
func TestInvalidateForcesReread(t *testing.T) {
	d, logPath, root := newStubbed(t, time.Hour, "")

	if _, err := d.Project(context.Background(), base(t, root)); err != nil {
		t.Fatalf("first Project: %v", err)
	}
	if n := callCount(t, logPath, "list"); n != 1 {
		t.Fatalf("expected one list call after the first read, got %d", n)
	}

	d.Invalidate(root)
	if _, err := d.Project(context.Background(), base(t, root)); err != nil {
		t.Fatalf("second Project: %v", err)
	}
	if n := callCount(t, logPath, "list"); n != 2 {
		t.Errorf("invalidation must force a re-read even within the TTL (list calls = %d)", n)
	}

	d.Invalidate("") // a no-op that must not panic
}

// TestProjectUnknownErrors verifies an address matching no discovered or
// cached project yields a not-found error and never reads anything.
func TestProjectUnknownErrors(t *testing.T) {
	d, logPath, _ := newStubbed(t, time.Hour, "")

	_, err := d.Project(context.Background(), "no-such-project")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a not-found error, got: %v", err)
	}
	if n := callCount(t, logPath, "list"); n != 0 {
		t.Errorf("unknown project must not trigger project reads (list calls = %d)", n)
	}
}

// TestInventoryConcurrentReads verifies `workmux list` and `workmux status`
// for one root run concurrently: with a second of sleep in each, the read
// stays well under two seconds.
func TestInventoryConcurrentReads(t *testing.T) {
	bin := t.TempDir()
	writeStub(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 1\n")
	writeStub(t, filepath.Join(bin, "git"), "#!/bin/sh\nexit 0\n")
	// sleep resolves via the shadowed PATH; delegate to the system binary by
	// absolute path so it still works there.
	writeStub(t, filepath.Join(bin, "sleep"), "#!/bin/sh\nexec /bin/sleep \"$@\"\n")
	logPath := filepath.Join(t.TempDir(), "calls.log")
	stub := `#!/bin/sh
{ printf 'CALL %s\n' "$1"; } >> "$GT_DISC_LOG"
case "$1" in list|status) sleep 1 ;; esac
case "$1" in
  list)      printf '%s' '[{"handle":"h1","branch":"b/h1","path":"/root/h1","is_main":true,"has_uncommitted_changes":false,"is_open":false,"created_at":1}]' ;;
  status)    printf '%s' '[]' ;;
  --version) echo 0.9-fake ;;
esac
exit 0
`
	writeStub(t, filepath.Join(bin, "wm"), stub)
	t.Setenv("PATH", bin)
	t.Setenv("GT_DISC_LOG", logPath)

	root := t.TempDir()
	d := New(Options{StartDir: root, Workmux: &workmux.Client{Bin: filepath.Join(bin, "wm")}, CacheTTL: time.Hour})

	start := time.Now()
	inv := d.Inventory(context.Background())
	if elapsed := time.Since(start); elapsed >= 2500*time.Millisecond {
		t.Errorf("list and status should run concurrently (serial would take two full seconds of sleep alone); inventory took %s", elapsed)
	}
	if len(inv.Projects) != 1 || inv.Projects[0].Worktrees[0].Handle != "h1" {
		t.Fatalf("unexpected inventory: %+v", inv.Projects)
	}
	if n := callCount(t, logPath, "list"); n != 1 {
		t.Errorf("expected one list call, got %d", n)
	}
	if n := callCount(t, logPath, "status"); n != 1 {
		t.Errorf("expected one status call, got %d", n)
	}
}

// TestInventoryIsolatesReadFailures preserves the per-root error semantics: a
// failed list yields an empty project carrying that error; a failed status
// keeps the listed worktrees and flags the error.
func TestInventoryIsolatesReadFailures(t *testing.T) {
	t.Run("list failure", func(t *testing.T) {
		d, _, _ := newStubbed(t, time.Hour, "list")
		inv := d.Inventory(context.Background())
		if len(inv.Projects) != 1 {
			t.Fatalf("expected the project to still be present, got %+v", inv.Projects)
		}
		p := inv.Projects[0]
		if p.Error == "" || !strings.Contains(p.Error, "boom-list") {
			t.Errorf("expected the list error on the project, got %q", p.Error)
		}
		if len(p.Worktrees) != 0 {
			t.Errorf("a failed list must yield an empty worktree set, got %+v", p.Worktrees)
		}
	})

	t.Run("status failure keeps listed worktrees", func(t *testing.T) {
		d, _, _ := newStubbed(t, time.Hour, "status")
		inv := d.Inventory(context.Background())
		if len(inv.Projects) != 1 {
			t.Fatalf("expected the project to still be present, got %+v", inv.Projects)
		}
		p := inv.Projects[0]
		if p.Error == "" || !strings.Contains(p.Error, "boom-status") {
			t.Errorf("expected the status error on the project, got %q", p.Error)
		}
		if len(p.Worktrees) != 1 || p.Worktrees[0].Handle != "h1" {
			t.Errorf("a failed status must keep listed worktrees, got %+v", p.Worktrees)
		}
	})
}

// TestInventoryReusesProbesWithinTTL verifies the root set, tmux availability,
// and version check are cached for one TTL window: a second inventory within
// the window performs no workmux reads at all.
func TestInventoryReusesProbesWithinTTL(t *testing.T) {
	d, logPath, _ := newStubbed(t, time.Hour, "")

	inv1 := d.Inventory(context.Background())
	if !inv1.WorkmuxAvailable || inv1.WorkmuxVersion != "0.9-fake" {
		t.Fatalf("unexpected first inventory: %+v", inv1)
	}
	if inv1.TmuxAvailable {
		t.Errorf("the stub tmux reports no server; TmuxAvailable should be false")
	}

	d.Inventory(context.Background()) // within the TTL window
	if n := callCount(t, logPath, "list"); n != 1 {
		t.Errorf("second inventory within TTL must reuse cached probes and projects (list calls = %d)", n)
	}
}
