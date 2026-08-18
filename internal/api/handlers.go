package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"gittreemux/internal/focus"
	"gittreemux/internal/tmux"
	"gittreemux/internal/workmux"
)

// findProject locates a project by its name (basename) or root path in a fresh
// inventory.
func (s *Server) findProject(ctx context.Context, name string) (*workmux.Project, error) {
	inv := s.Discoverer.Inventory(ctx)
	for i := range inv.Projects {
		p := &inv.Projects[i]
		if p.Name == name || p.Root == name || filepath.Base(p.Root) == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", name)
}

func getWorktree(p *workmux.Project, handle string) (*workmux.Worktree, bool) {
	for i := range p.Worktrees {
		if p.Worktrees[i].Handle == handle {
			return &p.Worktrees[i], true
		}
	}
	return nil, false
}

// handleProjects returns the cross-project inventory.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Discoverer.Inventory(r.Context()))
}

// handleWorktree returns a single worktree record within a project.
func (s *Server) handleWorktree(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	wt, ok := getWorktree(p, handle)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p.Name, "worktree": wt})
}

// handleOutput returns recent agent output for a worktree via lazy capture.
func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	wt, ok := getWorktree(p, handle)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}
	if wt.Agent == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("worktree %q has no running agent to capture output from", handle))
		return
	}

	out, err := s.Workmux.Capture(p.Root, handle)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p.Name, "handle": handle, "output": out})
}

// handleOpen opens (or focuses) the worktree's tmux window.
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	s.doAction(w, r, "open")
}

// handleClose closes the worktree's tmux window, preserving the worktree.
func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	s.doAction(w, r, "close")
}

// doAction runs a simple open/close action on the addressed worktree.
func (s *Server) doAction(w http.ResponseWriter, r *http.Request, action string) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if _, ok := getWorktree(p, handle); !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}

	var runErr error
	switch action {
	case "open":
		runErr = s.Workmux.Open(p.Root, handle)
	case "close":
		runErr = s.Workmux.Close(p.Root, handle)
	}
	s.log(action, project, handle, runErr)
	if runErr != nil {
		writeErr(w, http.StatusBadGateway, runErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": action, "project": p.Name, "handle": handle})
}

// focusWindowPrefix is the workmux tmux window name prefix used when
// recovering a session from live panes. It matches the default discovery
// prefix; the agent's own session is the primary source when available.
const focusWindowPrefix = "wm-"

// handleFocus focuses a worktree whose window is already open: it switches to
// the worktree's tmux window and best-effort brings the hosting terminal to the
// OS foreground. A failure to switch the window is a hard error; a failure to
// activate the terminal is reported in the response and does not fail the action.
func (s *Server) handleFocus(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	wt, ok := getWorktree(p, handle)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}
	if !wt.IsOpen {
		writeErr(w, http.StatusConflict, fmt.Errorf("worktree %q has no open window to focus", handle))
		return
	}

	if runErr := s.Workmux.Open(p.Root, handle); runErr != nil {
		s.log("focus", project, handle, runErr)
		writeErr(w, http.StatusBadGateway, runErr)
		return
	}

	session, paneID := resolveFocusTarget(wt, handle)
	var res focus.Result
	if session != "" {
		res = s.focusActivator().ActivateSession(session, paneID)
	} else {
		res = focus.Result{Activated: false, Note: "could not determine the tmux session to bring a terminal to the front"}
	}

	s.log("focus", project, handle, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"action":    "focus",
		"project":   p.Name,
		"handle":    handle,
		"activated": res.Activated,
		"app":       res.App,
		"note":      res.Note,
	})
}

// resolveFocusTarget determines the tmux session to activate for a worktree and
// a pane id within its window, which iTerm2 needs to identify the tab to
// surface. It prefers the worktree's agent session and falls back to matching
// the workmux window name against live panes. The session is empty when no
// session can be determined; the pane id is empty when only the session is
// known.
func resolveFocusTarget(wt *workmux.Worktree, handle string) (session, paneID string) {
	if wt.Agent != nil && wt.Agent.Session != "" {
		return wt.Agent.Session, wt.Agent.PaneID
	}
	panes, err := tmux.ListPanes()
	if err != nil {
		return "", ""
	}
	target := focusWindowPrefix + handle
	for _, p := range panes {
		if p.WindowName == target {
			return p.Session, p.ID
		}
	}
	return "", ""
}

// focusActivator returns the configured focus activator, or a default one.
func (s *Server) focusActivator() *focus.Activator {
	if s.Focus != nil {
		return s.Focus
	}
	return focus.New()
}

type sendBody struct {
	Text string `json:"text"`
}

// handleSend delivers a prompt to the worktree's running agent.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	wt, ok := getWorktree(p, handle)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}
	if wt.Agent == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("no agent is available in worktree %q to receive the prompt", handle))
		return
	}

	var body sendBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %v", err))
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("a non-empty prompt text is required"))
		return
	}

	runErr := s.Workmux.Send(p.Root, handle, body.Text)
	s.log("send", project, handle, runErr)
	if runErr != nil {
		writeErr(w, http.StatusBadGateway, runErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "send", "project": p.Name, "handle": handle})
}

type removeBody struct {
	Confirmed bool `json:"confirmed"`
}

// handleRemove removes the worktree, its window, and its branch. It requires an
// explicit confirmation flag and, when the worktree has uncommitted changes,
// surfaces a specific warning in the precondition response.
func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	handle := r.PathValue("handle")

	p, err := s.findProject(r.Context(), project)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	wt, ok := getWorktree(p, handle)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("worktree %q not found in project %q", handle, project))
		return
	}

	var body removeBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !body.Confirmed {
		warn := ""
		if wt.HasUncommittedChanges {
			warn = " WARNING: this worktree has uncommitted changes that will be permanently lost."
		}
		writeErr(w, http.StatusPreconditionRequired, fmt.Errorf(
			"removal of %q requires explicit confirmation (send {\"confirmed\": true}).%s", handle, warn))
		return
	}

	// -f is passed because there is no TTY for workmux's interactive prompt; the
	// destructive step is already gated by the confirmed flag above and by the
	// UI's confirmation dialog.
	runErr := s.Workmux.Remove(p.Root, handle, true)
	s.log("remove", project, handle, runErr)
	if runErr != nil {
		writeErr(w, http.StatusBadGateway, runErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "remove", "project": p.Name, "handle": handle})
}

// handleHealth is a trivial liveness probe.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) log(action, project, worktree string, err error) {
	if s.Log != nil {
		s.Log.Log(action, project, worktree, err)
	}
}
