// Package api exposes the JSON HTTP API and the embedded web UI over
// stdlib net/http. It is bound to loopback by default and stays stateless:
// every request reflects live local state via the Discoverer.
package api

import (
	"context"
	"net/http"

	"github.com/lwaddicor/gittreemux/internal/actionlog"
	"github.com/lwaddicor/gittreemux/internal/focus"
	"github.com/lwaddicor/gittreemux/internal/workmux"
	"github.com/lwaddicor/gittreemux/web"
)

// inventoryProvider abstracts the cross-project inventory so handlers can be
// tested with a fixed snapshot. *discover.Discoverer satisfies it.
type inventoryProvider interface {
	Inventory(ctx context.Context) *workmux.Inventory
}

// Server holds the collaborators shared by all handlers.
type Server struct {
	Discoverer inventoryProvider
	Workmux    *workmux.Client
	Log        *actionlog.Logger
	// Focus activates the terminal hosting a focused worktree. When nil a
	// default (exec.Run) activator is used.
	Focus *focus.Activator
}

// Routes builds the mux serving both the JSON API and the embedded web UI.
// It relies on Go 1.22+ method+path patterns for clean routing.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// JSON API.
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/projects/{project}/worktrees/{handle}", s.handleWorktree)
	mux.HandleFunc("GET /api/projects/{project}/worktrees/{handle}/output", s.handleOutput)
	mux.HandleFunc("POST /api/projects/{project}/worktrees/{handle}/open", s.handleOpen)
	mux.HandleFunc("POST /api/projects/{project}/worktrees/{handle}/close", s.handleClose)
	mux.HandleFunc("POST /api/projects/{project}/worktrees/{handle}/focus", s.handleFocus)
	mux.HandleFunc("POST /api/projects/{project}/worktrees/{handle}/send", s.handleSend)
	mux.HandleFunc("POST /api/projects/{project}/worktrees/{handle}/remove", s.handleRemove)
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Web UI (embedded) + assets; the more specific /api routes win first.
	mux.Handle("GET /", http.FileServer(http.FS(web.FS())))

	return mux
}
