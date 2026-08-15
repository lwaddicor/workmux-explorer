// Package tmux reads the live tmux server to enumerate windows and panes
// across every session. This is the global, cross-project source of truth for
// "what is running" that drives project discovery.
package tmux

import (
	"errors"
	"fmt"
	"strings"

	"gittreemux/internal/exec"
)

// ErrNotRunning is returned when the tmux binary is missing or no server is
// running (so there are no windows to enumerate).
var ErrNotRunning = errors.New("tmux is not available or no server is running")

// Pane is one tmux pane with the metadata we need for discovery.
type Pane struct {
	Session     string
	WindowIndex string
	WindowName  string
	Path        string
}

// listFormat uses tabs as separators; a tab will not appear in any of the
// fields we collect.
const listFormat = "#{session_name}\t#{window_index}\t#{window_name}\t#{pane_current_path}"

// ListPanes returns every pane across all tmux sessions.
func ListPanes() ([]Pane, error) {
	res := exec.Run("", "tmux", "list-panes", "-s", "-F", listFormat)
	if res.Err != nil {
		return nil, ErrNotRunning
	}
	if !res.OK() {
		return nil, ErrNotRunning
	}
	var panes []Pane
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		panes = append(panes, Pane{
			Session:     parts[0],
			WindowIndex: parts[1],
			WindowName:  parts[2],
			Path:        parts[3],
		})
	}
	return panes, nil
}

// Available reports whether a tmux server is currently reachable.
func Available() bool {
	res := exec.Run("", "tmux", "list-sessions")
	return res.OK()
}

// ErrString formats a tmux error for the user.
func ErrString(err error) string {
	if errors.Is(err, ErrNotRunning) {
		return "no tmux server is running, so no running worktrees could be found"
	}
	return fmt.Sprintf("tmux error: %v", err)
}
