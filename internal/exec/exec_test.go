package exec

import (
	"context"
	"testing"
	"time"
)

func TestRunCtxNormalCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := RunCtx(ctx, "", "echo", "hello")
	if res.Err != nil || !res.OK() {
		t.Fatalf("expected success, got %+v", res)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("unexpected stdout %q", res.Stdout)
	}
	if res.TimedOut {
		t.Errorf("TimedOut should be false for a command that finished in time")
	}
}

func TestRunCtxKillsOnDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	res := RunCtx(ctx, "", "sleep", "30")
	elapsed := time.Since(start)

	if !res.TimedOut {
		t.Fatalf("expected TimedOut=true, got %+v", res)
	}
	if res.Err != nil {
		t.Errorf("Err should be nil for a killed process (it is reserved for launch failures), got %v", res.Err)
	}
	if elapsed >= 2*time.Second {
		t.Errorf("process was not killed promptly: waited %s for a 30s sleep", elapsed)
	}
}

func TestRunCtxDeadlineAfterNormalExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := RunCtx(ctx, "", "false")
	if res.TimedOut {
		t.Errorf("TimedOut should be false when the command exits on its own before the deadline")
	}
	if res.Err != nil || res.ExitCode == 0 {
		t.Fatalf("expected a non-zero exit without Err, got %+v", res)
	}
}

func TestRunCtxMissingBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := RunCtx(ctx, "", "definitely-not-a-real-binary-xyz")
	if res.Err == nil {
		t.Fatalf("expected an Err for a missing binary, got %+v", res)
	}
	if res.TimedOut {
		t.Errorf("a launch failure must not be reported as a timeout")
	}
}

func TestRunWrapsWithoutDeadline(t *testing.T) {
	res := Run("", "echo", "hi")
	if !res.OK() || res.Stdout != "hi\n" || res.TimedOut {
		t.Fatalf("Run should behave as before, got %+v", res)
	}
}
