package tmux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stubTmux puts a fake `tmux` binary on PATH so tests never touch a real tmux
// server.
func stubTmux(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub tmux: %v", err)
	}
	t.Setenv("PATH", bin)
}

func TestListPanesCtxParsesPanes(t *testing.T) {
	stubTmux(t, "#!/bin/sh\nprintf 's1\\t0\\tmain\\t%%0\\t/tmp\\n'\nprintf 's1\\t1\\twm-feat-x\\t%%1\\t/tmp/feat-x\\n'\n")
	panes, err := ListPanesCtx(context.Background())
	if err != nil {
		t.Fatalf("ListPanesCtx: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d: %+v", len(panes), panes)
	}
	if panes[1].Session != "s1" || panes[1].WindowIndex != "1" || panes[1].WindowName != "wm-feat-x" || panes[1].ID != "%1" || panes[1].Path != "/tmp/feat-x" {
		t.Errorf("unexpected pane fields: %+v", panes[1])
	}
}

func TestListPanesCtxNoServer(t *testing.T) {
	stubTmux(t, "#!/bin/sh\nexit 1\n")
	_, err := ListPanesCtx(context.Background())
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("expected ErrNotRunning for a non-zero exit, got %v", err)
	}
}

// TestListPanesCtxTimesOut verifies a wedged tmux server does not outlive the
// caller's deadline: the query is abandoned promptly and reported as
// not-running.
func TestListPanesCtxTimesOut(t *testing.T) {
	stubTmux(t, "#!/bin/sh\nexec /bin/sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := ListPanesCtx(ctx)
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("expected ErrNotRunning, got %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 5*time.Second {
		t.Fatalf("the wedged query must not outlive the deadline, took %s", elapsed)
	}
}
