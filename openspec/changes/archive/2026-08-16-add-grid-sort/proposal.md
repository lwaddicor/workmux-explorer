## Why

The worktree grid currently shows worktrees in whatever order the inventory returns them, so there is no way to quickly find, for example, the most recently created worktree. Users need to scan every card to locate what they want.

## What Changes

- Add a sort control to the web UI grid view letting the user order the selected repository's worktrees by a chosen field and direction.
- Provide at least these sort options: created (newest first), created (oldest first), name (A→Z), name (Z→A). "Created" sorts on the worktree's `created_at`; "name" sorts on the handle.
- Default sort is created, newest first; the choice is remembered across page loads (localStorage, same pattern as the poll interval and selected project).
- Sorting is client-side only; it does not change the JSON API response or the order of data returned by the server.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `web-dashboard`: the "Serve a web UI and a JSON API" requirement gains a sort control over the worktree grid's ordering; the grid ordering becomes user-selectable while the API order is unchanged.

## Impact

- `web/index.html` — new sort selector in the grid controls area.
- `web/app.js` — sort state, comparison logic, and re-render on change.
- `web/style.css` — styling for the control.
- No Go code, API, or dependency changes.
