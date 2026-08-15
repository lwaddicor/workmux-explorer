# AGENTS.md

## Project

gittreemux is a single, self-contained Go binary that surfaces every workmux
worktree and agent on the machine as a local web dashboard (loopback by
default), with actions (open/close/send/remove) from the browser.

- Go 1.26, **stdlib only** — no external dependencies. Do not add any
  without explicit justification.
- `cmd/gittreemux/` — CLI entrypoint (`serve`, `version`).
- `internal/api` — JSON HTTP API + embedded web UI (Go 1.22+ method+path routing).
- `internal/discover` — cross-project worktree discovery, caching.
- `internal/workmux` — client for the `workmux` binary.
- `internal/tmux`, `internal/exec`, `internal/actionlog` — supporting packages.
- `web/` — vanilla JS/CSS, embedded via `go:embed`. **No framework, no
  build step.** Keep it that way.
- Loopback-only binding is intentional; don't "fix" it into a public server.

## Commands

```bash
go build ./...          # build
go test ./...           # test
go vet ./...            # vet
gofmt -l .              # must return nothing
go run ./cmd/gittreemux serve   # run (defaults: 127.0.0.1:8787)
```

## Conventional Commits (required)

Every commit MUST follow Conventional Commits:

```
type: summary
```

- Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`.
- Optional scope allowed: `feat(api): ...`.
- Imperative mood, lowercase, no trailing period, summary ≤ ~72 chars.
- Examples from history: `feat: add web dashboard`, `docs: add OpenSpec specs`.
- One logical change per commit; do not mix refactors with behavior changes.

## OpenSpec workflow (required for non-trivial work)

This repo is developed spec-driven with OpenSpec. Layout:

- `openspec/specs/` — current source of truth (worktree-inventory,
  worktree-lifecycle, agent-interaction, web-dashboard).
- `openspec/changes/` — active change proposals; `openspec/changes/archive/`
  — completed ones.

For any feature, significant behavior change, or API change:

1. **Propose** — use the `openspec-propose` skill (or `/opsx-propose`) to
   create the change (proposal, design, delta specs, tasks).
2. **Implement** — use `openspec-apply-change` (`/opsx-apply`) to work
   through the tasks; mark them off in `tasks.md` as they complete.
3. **Sync** — use `openspec-sync-specs` (`/opsx-sync`) to fold delta specs
   into `openspec/specs/`.
4. **Archive** — use `openspec-archive-change` (`/opsx-archive`) when done.

For ambiguous requirements, use `openspec-explore` (`/opsx-explore`) to think
it through before proposing. To revise an existing change, use
`openspec-update-change` (`/opsx-update`).

Trivial changes (typos, one-line fixes, comments) may skip the workflow, but
the resulting behavior must still conform to `openspec/specs/`.

## Worktrees & workmux

Parallel development happens in workmux worktrees (git worktree + tmux
window, named `wm-<handle>`).

- **Dispatch** — write a prompt file, then `workmux add -b -P <file>`.
  Don't research the codebase before dispatching; the worktree agent does that.
- **Monitor** — `workmux status`, `workmux wait <handle>`,
  `workmux capture <handle>`, `workmux send <handle> "msg"`.
- **Running the app** — the main worktree owns `127.0.0.1:8787`. When
  developing in a worktree, run your instance on a different port so both
  work: `go run ./cmd/gittreemux serve -listen 127.0.0.1:8788` (8788+ for
  worktrees). Don't assume 8787 is free; don't kill the main instance.
- **Finish (standard flow)** — commit with conventional commits,
  `git push -u origin HEAD`, open a PR (use the `open-pr` skill /
  `/open-pr`), then after the PR merges remove the worktree:
  `workmux remove <handle>` (or `workmux rm --gone`).
- Never use `workmux merge`; the PR flow is the only finishing path.
- This repo is itself typically edited inside workmux worktrees — check
  `workmux list` to see what is active.

## Code conventions

- Doc comments on all exported symbols, describing the public contract.
- Small, focused packages under `internal/`; no cycles.
- Errors: wrap with `fmt.Errorf("...: %w", err)` at boundaries.
- Tests live next to code (`*_test.go`); keep them hermetic (no real tmux
  or network in unit tests — see how `internal/workmux` stubs its binary).
- Web UI: no dependencies, no bundler; plain `fetch` against the JSON API.
