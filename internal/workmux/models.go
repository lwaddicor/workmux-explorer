package workmux

// Status is the high-level state of a running agent as reported by workmux.
type Status string

const (
	StatusWorking Status = "working"
	StatusDone    Status = "done"
	StatusWaiting Status = "waiting"
)

// GitState is the git status of a worktree's branch relative to its base.
type GitState struct {
	HasStaged          bool `json:"has_staged"`
	HasUnstaged        bool `json:"has_unstaged"`
	HasUnmergedCommits bool `json:"has_unmerged_commits"`
}

// AgentStatus is a single active agent as reported by `workmux status --json --git`.
type AgentStatus struct {
	Worktree    string   `json:"worktree"`
	Branch      string   `json:"branch,omitempty"`
	Status      Status   `json:"status"`
	ElapsedSecs int64    `json:"elapsed_secs"`
	Title       string   `json:"title,omitempty"`
	AgentKind   *string  `json:"agent_kind"`
	PaneID      string   `json:"pane_id,omitempty"`
	Workdir     string   `json:"workdir,omitempty"`
	Session     string   `json:"session,omitempty"`
	WindowName  string   `json:"window_name,omitempty"`
	UpdatedTS   int64    `json:"updated_ts,omitempty"`
	Git         GitState `json:"git"`
}

// Worktree is a single worktree from `workmux list --json`, enriched with the
// active agent (if any) for that handle.
type Worktree struct {
	Handle                string `json:"handle"`
	Branch                string `json:"branch"`
	Path                  string `json:"path"`
	IsMain                bool   `json:"is_main"`
	Mode                  string `json:"mode,omitempty"`
	HasUncommittedChanges bool   `json:"has_uncommitted_changes"`
	IsOpen                bool   `json:"is_open"`
	CreatedAt             int64  `json:"created_at"`

	// Agent is non-nil only when workmux reports an active agent for this
	// worktree (i.e. it appears in `workmux status`).
	Agent *AgentStatus `json:"agent,omitempty"`
}

// Project is the unified, per-project record: a project root and every
// worktree workmux knows about for it, joined with live agent status.
type Project struct {
	Name      string     `json:"name"`
	Root      string     `json:"root"`
	Worktrees []Worktree `json:"worktrees"`
	// Error is set when this project could not be fully read; the inventory
	// still returns the other, readable projects.
	Error string `json:"error,omitempty"`
}

// Inventory is the cross-project snapshot returned by the API.
type Inventory struct {
	GeneratedAt      int64  `json:"generated_at"`
	TmuxAvailable    bool   `json:"tmux_available"`
	WorkmuxAvailable bool   `json:"workmux_available"`
	WorkmuxVersion   string `json:"workmux_version,omitempty"`
	// Degraded is a human-readable reason when the inventory is empty or
	// partial due to missing dependencies. Empty when healthy.
	Degraded string    `json:"degraded,omitempty"`
	Projects []Project `json:"projects"`
}
