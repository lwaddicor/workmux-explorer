package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lwaddicor/gittreemux/internal/exec"
	"github.com/lwaddicor/gittreemux/internal/focus"
	"github.com/lwaddicor/gittreemux/internal/workmux"
)

// fakeInventoryProvider returns a fixed inventory so handlers can be tested
// without a real tmux server, git repository, or workmux reads.
type fakeInventoryProvider struct {
	inv *workmux.Inventory
}

func (f *fakeInventoryProvider) Inventory(context.Context) *workmux.Inventory {
	return f.inv
}

func testInventory(isOpen bool, session, root string) *workmux.Inventory {
	return &workmux.Inventory{
		Projects: []workmux.Project{{
			Name: "demo",
			Root: root,
			Worktrees: []workmux.Worktree{{
				Handle: "feat-x",
				Branch: "feat/x",
				IsOpen: isOpen,
				Agent:  &workmux.AgentStatus{Worktree: "feat-x", Status: workmux.StatusWorking, Session: session},
			}},
		}},
	}
}

// newFakeWorkmuxBin writes a stub workmux binary that records its argv and
// exits with the given code, so the handler's Open call can be controlled
// without invoking a real workmux.
func newFakeWorkmuxBin(t *testing.T, exitCode int) (binPath, logPath string) {
	t.Helper()
	dir := t.TempDir()
	binPath = filepath.Join(dir, "fake-workmux")
	logPath = filepath.Join(dir, "args.log")
	script := fmt.Sprintf(`#!/bin/sh
{
  for a in "$@"; do printf '%%s ' "$a"; done
  printf '\n'
} >> "$GT_FAKE_LOG"
exit %d
`, exitCode)
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake workmux: %v", err)
	}
	t.Setenv("GT_FAKE_LOG", logPath)
	return binPath, logPath
}

func newTestServer(t *testing.T, inv *workmux.Inventory, wmExit int, foc *focus.Activator) (*Server, string) {
	t.Helper()
	binPath, logPath := newFakeWorkmuxBin(t, wmExit)
	s := &Server{
		Discoverer: &fakeInventoryProvider{inv: inv},
		Workmux:    &workmux.Client{Bin: binPath},
		Focus:      foc,
	}
	return s, logPath
}

func postFocus(t *testing.T, s *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/projects/demo/worktrees/feat-x/focus", nil)
	rr := httptest.NewRecorder()
	s.Routes().ServeHTTP(rr, req)
	return rr
}

// fakeDetachedFocus reports no attached client, so ActivateSession degrades to
// a best-effort "could not bring a terminal to the front" outcome.
func fakeDetachedFocus() *focus.Activator {
	return &focus.Activator{
		GOOS: "darwin",
		Run: func(dir, name string, args ...string) exec.Result {
			if name == "tmux" {
				return exec.Result{Stdout: "", ExitCode: 0}
			}
			return exec.Result{ExitCode: 0}
		},
	}
}

func TestHandleFocusWindowNotOpen(t *testing.T) {
	s, logPath := newTestServer(t, testInventory(false, "0", t.TempDir()), 0, nil)
	rr := postFocus(t, s)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a closed window, got %d: %s", rr.Code, rr.Body.String())
	}
	if b, _ := os.ReadFile(logPath); b != nil && len(b) > 0 {
		t.Errorf("Open should not be invoked for a closed window, but the fake workmux logged: %s", b)
	}
}

func TestHandleFocusSwitchFails(t *testing.T) {
	s, logPath := newTestServer(t, testInventory(true, "0", t.TempDir()), 1, nil)
	rr := postFocus(t, s)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when the window switch fails, got %d: %s", rr.Code, rr.Body.String())
	}
	// The 502 must come from the window switch actually running and failing.
	b, _ := os.ReadFile(logPath)
	if !strings.Contains(string(b), "open") {
		t.Errorf("expected the fake workmux to be invoked with 'open', logged: %s", b)
	}
}

func TestHandleFocusBestEffort(t *testing.T) {
	s, _ := newTestServer(t, testInventory(true, "0", t.TempDir()), 0, fakeDetachedFocus())
	rr := postFocus(t, s)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for a best-effort focus, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("ok should be true, got %v", body["ok"])
	}
	if body["action"] != "focus" {
		t.Errorf("action should be focus, got %v", body["action"])
	}
	if body["project"] != "demo" {
		t.Errorf("project should be demo, got %v", body["project"])
	}
	if body["handle"] != "feat-x" {
		t.Errorf("handle should be feat-x, got %v", body["handle"])
	}
	if body["activated"] != false {
		t.Errorf("activated should be false for a degraded focus, got %v", body["activated"])
	}
	if note, _ := body["note"].(string); note == "" {
		t.Errorf("note should be set when activation is not performed")
	}
}
