// Package workmux is a thin client over the `workmux` CLI. It shells out for
// every read and write so that workmux's own lifecycle behavior (hooks, file
// ops, branch cleanup) stays the source of truth.
package workmux

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lwaddicor/workmux-explorer/internal/exec"
)

// ErrNotInstalled is returned when the workmux binary cannot be found.
var ErrNotInstalled = errors.New("workmux binary not found on PATH")

// namePattern is the strict allowlist a worktree handle must match before it is
// ever passed to a workmux command. Handles observed in the wild use lowercase
// letters, digits, and the separators '-', '.', '_', '+'.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)

// ValidateName enforces the handle allowlist. It is defense-in-depth on top of
// passing values as discrete argv elements (no shell).
func ValidateName(name string) error {
	if name == "" {
		return errors.New("worktree name is empty")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid worktree name %q (allowed: letters, digits, . _ + -)", name)
	}
	return nil
}

// Client invokes the workmux binary.
type Client struct {
	// Bin is the workmux executable name or path. Defaults to "workmux".
	Bin string
}

// New returns a Client using the workmux binary on PATH.
func New() *Client { return &Client{Bin: "workmux"} }

func (c *Client) bin() string {
	if c.Bin == "" {
		return "workmux"
	}
	return c.Bin
}

func (c *Client) run(dir string, args ...string) exec.Result {
	return exec.Run(dir, c.bin(), args...)
}

// runOK executes a mutating command and reports success (nil) or a descriptive
// error. It distinguishes a missing binary from a non-zero exit.
func (c *Client) runOK(dir string, args ...string) error {
	res := c.run(dir, args...)
	if res.Err != nil {
		return ErrNotInstalled
	}
	if !res.OK() {
		return fail(res)
	}
	return nil
}

// fail maps a failed run to a descriptive error, distinguishing a missing
// binary from a non-zero exit.
func fail(res exec.Result) error {
	if res.Err != nil {
		return ErrNotInstalled
	}
	msg := strings.TrimSpace(res.Stderr)
	if msg == "" {
		msg = strings.TrimSpace(res.Stdout)
	}
	if msg == "" {
		msg = fmt.Sprintf("exited with code %d", res.ExitCode)
	}
	return errors.New(msg)
}

// List returns every worktree workmux knows about for the project rooted at dir.
func (c *Client) List(dir string) ([]Worktree, error) {
	res := c.run(dir, "list", "--json")
	if res.Err != nil {
		return nil, ErrNotInstalled
	}
	if !res.OK() {
		return nil, fail(res)
	}
	return parseList([]byte(res.Stdout))
}

// Status returns the active agents (with git state) for the project rooted at dir.
func (c *Client) Status(dir string) ([]AgentStatus, error) {
	res := c.run(dir, "status", "--json", "--git")
	if res.Err != nil {
		return nil, ErrNotInstalled
	}
	if !res.OK() {
		return nil, fail(res)
	}
	return parseStatus([]byte(res.Stdout))
}

// parseList decodes `workmux list --json` output.
func parseList(b []byte) ([]Worktree, error) {
	var wt []Worktree
	if err := json.Unmarshal(b, &wt); err != nil {
		return nil, fmt.Errorf("parse workmux list: %w", err)
	}
	return wt, nil
}

// parseStatus decodes `workmux status --json --git` output.
func parseStatus(b []byte) ([]AgentStatus, error) {
	var st []AgentStatus
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("parse workmux status: %w", err)
	}
	return st, nil
}

// Capture returns the recent terminal output of the named worktree's agent.
func (c *Client) Capture(dir, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	res := c.run(dir, "capture", name)
	if res.Err != nil {
		return "", ErrNotInstalled
	}
	if !res.OK() {
		return "", fail(res)
	}
	return res.Stdout, nil
}

// Open opens or switches to the tmux window of the named worktree.
func (c *Client) Open(dir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return c.runOK(dir, "open", name)
}

// Close closes the named worktree's tmux window, keeping the worktree and branch.
func (c *Client) Close(dir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return c.runOK(dir, "close", name)
}

// Remove deletes the named worktree, its tmux window, and its local branch.
// When force is true it is passed -f (skip workmux's interactive confirmation
// and ignore uncommitted changes); the caller is responsible for having already
// surfaced an explicit confirmation to the user.
func (c *Client) Remove(dir, name string, force bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	args := []string{"remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)
	return c.runOK(dir, args...)
}

// Send delivers a prompt to the named worktree's running agent.
func (c *Client) Send(dir, name, text string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return c.runOK(dir, "send", name, text)
}

// Version returns the workmux version string, or an error if it is missing.
func (c *Client) Version() (string, error) {
	res := c.run("", "--version")
	if res.Err != nil {
		return "", ErrNotInstalled
	}
	if !res.OK() {
		return "", fail(res)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// Join attaches each active agent to its worktree by handle, returning the
// unified records. Worktrees without an active agent keep Agent == nil.
func Join(worktrees []Worktree, statuses []AgentStatus) []Worktree {
	byHandle := make(map[string]*AgentStatus, len(statuses))
	for i := range statuses {
		byHandle[statuses[i].Worktree] = &statuses[i]
	}
	out := make([]Worktree, 0, len(worktrees))
	for i := range worktrees {
		wt := worktrees[i]
		if ag, ok := byHandle[wt.Handle]; ok {
			wt.Agent = ag
		}
		out = append(out, wt)
	}
	return out
}
