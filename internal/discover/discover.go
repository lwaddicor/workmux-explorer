// Package discover builds the cross-project worktree inventory by (1) finding
// every project root that is running in tmux, (2) reading each project's
// worktrees and active agents concurrently, and (3) joining them into unified
// per-worktree records. Per-project failures are isolated so one unreadable
// project does not fail the whole inventory. Reads are bounded by deadlines: a
// wedged probe or project degrades to an error on that scope instead of
// outliving its query, and recent results (roots, version, per-project) are
// reused briefly within the cache TTL.
package discover

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lwaddicor/workmux-explorer/internal/exec"
	"github.com/lwaddicor/workmux-explorer/internal/tmux"
	"github.com/lwaddicor/workmux-explorer/internal/workmux"
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
	// CacheTTL is how long a per-project result — and the discovery root set,
	// tmux availability, and workmux version it was built from — are reused
	// before re-reading. It should not be shorter than the UI's poll interval
	// or action lookups will always miss.
	CacheTTL time.Duration
	// Workmux is the client used for reads. Defaults to a new client; its
	// Timeout also bounds every discovery probe (tmux, git) and workmux read.
	Workmux *workmux.Client
}

type cacheEntry struct {
	at      time.Time
	project workmux.Project
}

// metaCache is the brief snapshot of discovery-level probes: the project root
// set, tmux availability, and the workmux version check. Reusing it within one
// TTL window lets repeated inventories skip re-probing entirely.
type metaCache struct {
	at        time.Time
	roots     []string
	tmuxOK    bool
	workmuxOK bool
	version   string
}

// Discoverer builds inventories and caches per-project reads briefly.
type Discoverer struct {
	opts  *Options
	mu    sync.Mutex
	cache map[string]cacheEntry // per-project results, keyed by root
	meta  metaCache             // discovery probes; zero at means not cached yet
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
		opts.CacheTTL = 5 * time.Second
	}
	if opts.Workmux == nil {
		opts.Workmux = workmux.New()
	}
	return &Discoverer{opts: &opts, cache: make(map[string]cacheEntry)}
}

// probeContext bounds the discovery probes (tmux list-panes, per-pane git
// rev-parse) by the client's read timeout on top of the caller's context, so a
// wedged probe cannot outlive its query. A zero client timeout leaves only the
// caller's deadline in play.
func (d *Discoverer) probeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if d.opts.Workmux.Timeout > 0 {
		return context.WithTimeout(ctx, d.opts.Workmux.Timeout)
	}
	return ctx, func() {}
}

// resolveProjectRoot maps a directory (or a linked worktree inside it) to the
// main repository root that owns it, using git plumbing so it does not depend
// on workmux. It returns ok=false when dir is not part of a git repository or
// the probe exceeds its deadline.
func resolveProjectRoot(ctx context.Context, dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	res := exec.RunCtx(ctx, dir, "git", "rev-parse", "--git-common-dir")
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
// Every probe runs under ctx so none can outlive the caller's deadline.
func (d *Discoverer) discoverRoots(ctx context.Context) ([]string, bool) {
	roots := make(map[string]bool)

	tmuxOK := false
	panes, err := tmux.ListPanesCtx(ctx)
	if err == nil {
		tmuxOK = true
		for _, p := range panes {
			if root, ok := resolveProjectRoot(ctx, p.Path); ok {
				roots[root] = true
			}
		}
	}

	if d.opts.StartDir != "" {
		if root, ok := resolveProjectRoot(ctx, d.opts.StartDir); ok {
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

func (d *Discoverer) cachedMeta() (*metaCache, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.meta.at.IsZero() || time.Since(d.meta.at) > d.opts.CacheTTL {
		return nil, false
	}
	m := d.meta
	return &m, true
}

func (d *Discoverer) storeMeta(m metaCache) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m.at = time.Now()
	d.meta = m
}

// readProject builds the unified record for one project root. `workmux list`
// and `workmux status --git` run concurrently; their failures are captured on
// the returned Project rather than propagated, so one bad project does not
// fail the inventory: a failed list yields an empty project with that error,
// while a failed status keeps the listed worktrees and flags the error.
func (d *Discoverer) readProject(root string) workmux.Project {
	if p, ok := d.cached(root); ok {
		return p
	}

	p := workmux.Project{
		Name:      filepath.Base(root),
		Root:      root,
		Worktrees: []workmux.Worktree{},
	}

	var (
		wts  []workmux.Worktree
		lErr error
		sts  []workmux.AgentStatus
		sErr error
		wg   sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		wts, lErr = d.opts.Workmux.List(root)
	}()
	go func() {
		defer wg.Done()
		sts, sErr = d.opts.Workmux.Status(root)
	}()
	wg.Wait()

	if lErr != nil {
		p.Error = lErr.Error()
		d.store(root, p)
		return p
	}
	if sErr != nil {
		p.Error = sErr.Error()
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

// matchRoot reports whether the project rooted at root is addressed by name —
// its base name or its full path. The rule mirrors how project names are
// derived so scoped and full-inventory lookups agree.
func matchRoot(root, name string) bool {
	return name != "" && (root == name || filepath.Base(root) == name)
}

// Project returns the unified record of one addressed project without building
// the full cross-project inventory: a fresh cached entry is returned as-is, a
// stale one re-reads just that root, and an uncached address runs discovery
// once to find its root. It errors when no discovered or cached project is
// addressed by name (project name or root path).
func (d *Discoverer) Project(ctx context.Context, name string) (*workmux.Project, error) {
	if p := d.cachedProjectByName(name); p != nil {
		return p, nil
	}

	probeCtx, cancel := d.probeContext(ctx)
	defer cancel()
	roots, _ := d.discoverRoots(probeCtx)

	for _, root := range roots {
		if matchRoot(root, name) {
			p := d.readProject(root)
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", name)
}

// cachedProjectByName returns the project addressed by name (base name or root
// path): a fresh entry is returned as-is, and a stale one re-reads just that
// root. It is nil when no cached project matches; first-match ordering follows
// sorted roots so it agrees with full-inventory lookups.
func (d *Discoverer) cachedProjectByName(name string) *workmux.Project {
	d.mu.Lock()
	var candidate string
	for root := range d.cache {
		if matchRoot(root, name) && (candidate == "" || root < candidate) {
			candidate = root
		}
	}
	if candidate == "" {
		d.mu.Unlock()
		return nil
	}
	entry := d.cache[candidate]
	fresh := time.Since(entry.at) <= d.opts.CacheTTL
	d.mu.Unlock()
	if fresh {
		p := entry.project
		return &p
	}
	p := d.readProject(candidate)
	return &p
}

// Inventory builds the current cross-project snapshot. The discovery probes
// and workmux version check are reused from a recent snapshot within the cache
// TTL; per-project reads run concurrently under a bounded pool.
func (d *Discoverer) Inventory(ctx context.Context) *workmux.Inventory {
	probeCtx, cancel := d.probeContext(ctx)

	var roots []string
	tmuxOK, workmuxOK, ver := false, false, ""
	if m, ok := d.cachedMeta(); ok {
		roots, tmuxOK, workmuxOK, ver = m.roots, m.tmuxOK, m.workmuxOK, m.version
	} else {
		// Detect workmux once so we can report a clear degraded reason.
		v, err := d.opts.Workmux.Version()
		if err == nil {
			workmuxOK = true
			ver = v
		}
		roots, tmuxOK = d.discoverRoots(probeCtx)
		d.storeMeta(metaCache{roots: roots, tmuxOK: tmuxOK, workmuxOK: workmuxOK, version: ver})
	}
	cancel()

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
