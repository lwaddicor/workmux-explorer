## Why

The dashboard renders every project as a long vertical section, so with several repositories the page becomes a single massive list: finding a specific worktree requires scrolling past all earlier projects.

## What Changes

- Replace the single vertical list with a two-pane layout: a left sidebar listing all repositories (projects) and a right pane showing the selected repository's worktrees.
- The sidebar shows each repository with a summary (worktree count, active-agent count) and a selection state; clicking a repository switches the right pane to it.
- Worktrees in the right pane are rendered as a responsive card grid instead of a stacked column.
- Preserve all existing worktree card content and actions (open/close window, send prompt, output, remove), the status badge, polling controls, degraded banner, and detail/send modals.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `web-dashboard`: the "Serve a web UI and a JSON API" requirement's presentation changes from a flat per-project list to a repository sidebar plus a per-repository worktree grid.

## Impact

- `web/index.html`: new sidebar/main layout markup.
- `web/app.js`: rendering split into sidebar and grid panes; selection state handling.
- `web/style.css`: two-pane layout and grid card styles.
- No API, backend (`internal/`, `cmd/`), or dependency changes.
