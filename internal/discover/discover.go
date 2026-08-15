// Package discover builds the cross-project worktree inventory by (1) finding
// every project root that is running in tmux, (2) reading each project's
// worktrees and active agents concurrently, and (3) joining them into unified
// per-worktree records. Per-project failures are isolated so one unreadable
// project does not fail the whole inventory.
package discover

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gittreemux/internal/exec"
	"gittreemux/internal/tmux"
	"gittreemux/internal/workmux"
)

// Options configures a Discoverer. Zero values fall back to sensible defaults.
type Options struct {
	// StartDir is the directory the server was started in; it is added as a
	// fallback project source so the started-from project is always included.
	StartDir string
	// Prefix is the workmux tmux window name prefix (default "wm-"). It is an
	// auxiliary discovery signal; path resolution is the primary one.
	Prefix string
	// Concurrency bounds the worker pool used to read projects in parallel.
	Concurrency int
	// CacheTTL is how long a per-project result is reused before re-reading.
	CacheTTL time.Duration
	// Workmux is the client used for reads. Defaults to a new client.
	Workmux *workmux.Client
}

type cacheEntry struct {
	at      time.Time
	project workmux.Project
}

// Discoverer builds inventories and caches per-project reads briefly.
type Discoverer struct {
	opts  *Options
	mu    sync.Mutex
	cache map[string]cacheEntry
}

// New returns a Discoverer with defaults applied.
func New(opts Options) *Discoverer {
	if opts.Prefix == "" {
		opts.Prefix = "wm-"
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 8
	}
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 2 * time.Second
	}
	if opts.Workmux == nil {
		opts.Workmux = workmux.New()
	}
	return &Discoverer{opts: &opts, cache: make(map[string]cacheEntry)}
}

// resolveProjectRoot maps a directory (or a linked worktree inside it) to the
// main repository root that owns it, using git plumbing so it does not depend
// on workmux. It returns ok=false when dir is not part of a git repository.
func resolveProjectRoot(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	res := exec.Run(dir, "git", "rev-parse", "--git-common-dir")
	if !res.OK() {
		return "", false
	}
	commonDir := strings.TrimSpace(res.Stdout)
	switch {
	case commonDir == "" || commonDir == ".git":
		return dir, true
	case filepath.IsAbs(commonDir):
		return filepath.Dir(commonDir), true
	default:
		return filepath.Dir(filepath.Join(dir, commonDir)), true
	}
}

// discoverRoots collects the de-duplicated, sorted set of project roots to read:
// every git repository surfaced by a tmux pane, plus the server's start
// directory as a fallback. It also reports whether a tmux server is reachable.
func (d *Discoverer) discoverRoots() ([]string, bool) {
	roots := make(map[string]bool)

	tmuxOK := false
	panes, err := tmux.ListPanes()
	if err == nil {
		tmuxOK = true
		for _, p := range panes {
			if root, ok := resolveProjectRoot(p.Path); ok {
				roots[root] = true
			}
		}
	}

	if d.opts.StartDir != "" {
		if root, ok := resolveProjectRoot(d.opts.StartDir); ok {
			roots[root] = true
		}
	}

	out := make([]string, 0, len(roots))
	for r := range roots {
		out = append(out, r)
	}
	sort.Strings(out)
	return out, tmuxOK
}

// readProject builds the unified record for one project root, reading
// `workmux list` and `workmux status` and joining them. Failures are captured
// on the returned Project rather than propagated, so one bad project does not
// fail the inventory.
func (d *Discoverer) readProject(root string) workmux.Project {
	if p, ok := d.cached(root); ok {
		return p
	}

	p := workmux.Project{
		Name:      filepath.Base(root),
		Root:      root,
		Worktrees: []workmux.Worktree{},
	}

	wts, listErr := d.opts.Workmux.List(root)
	if listErr != nil {
		p.Error = listErr.Error()
		d.store(root, p)
		return p
	}

	sts, statusErr := d.opts.Workmux.Status(root)
	if statusErr != nil {
		p.Error = statusErr.Error()
	}
	p.Worktrees = workmux.Join(wts, sts)

	d.store(root, p)
	return p
}

func (d *Discoverer) cached(root string) (workmux.Project, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.cache[root]
	if !ok || time.Since(e.at) > d.opts.CacheTTL {
		return workmux.Project{}, false
	}
	return e.project, true
}

func (d *Discoverer) store(root string, p workmux.Project) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache[root] = cacheEntry{at: time.Now(), project: p}
}

// Inventory builds the current cross-project snapshot.
func (d *Discoverer) Inventory(ctx context.Context) *workmux.Inventory {
	roots, tmuxOK := d.discoverRoots()

	// Detect workmux once so we can report a clear degraded reason.
	workmuxOK := false
	ver := ""
	if v, err := d.opts.Workmux.Version(); err == nil {
		workmuxOK = true
		ver = v
	}

	results := make([]workmux.Project, len(roots))
	sem := make(chan struct{}, d.opts.Concurrency)
	var wg sync.WaitGroup
	for i, root := range roots {
		wg.Add(1)
		go func(i int, root string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			results[i] = d.readProject(root)
		}(i, root)
	}
	wg.Wait()

	projects := make([]workmux.Project, 0, len(roots))
	for _, p := range results {
		if p.Root == "" {
			continue
		}
		projects = append(projects, p)
	}

	inv := &workmux.Inventory{
		GeneratedAt:      time.Now().Unix(),
		TmuxAvailable:    tmuxOK,
		WorkmuxAvailable: workmuxOK,
		WorkmuxVersion:   ver,
		Projects:         projects,
	}
	inv.Degraded = d.degradedReason(tmuxOK, workmuxOK, len(projects))
	return inv
}

func (d *Discoverer) degradedReason(tmuxOK, workmuxOK bool, nProjects int) string {
	if !workmuxOK {
		return "the workmux CLI was not found on PATH; install workmux to see its worktrees"
	}
	if nProjects == 0 {
		if !tmuxOK {
			return "no tmux server is running and no projects were found, so there are no worktrees to show"
		}
		return "no workmux worktrees were found on this machine"
	}
	return ""
}
