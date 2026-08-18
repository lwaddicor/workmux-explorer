## Context

See proposal.md for motivation. The grid is rendered client-side in `web/app.js` (`renderGrid` builds HTML from `p.worktrees` of the selected project). Each worktree already carries a `created_at` (unix seconds) and a `handle`. The UI persists lightweight preferences (poll interval, selected project) in `localStorage`. The Go server returns worktrees in a fixed order and has no concept of a presentation sort.

## Goals / Non-Goals

**Goals:**
- Let the user reorder the selected repository's worktree cards in the grid.
- Support the four orderings from the spec: created newest-first, created oldest-first, name A→Z, name Z→A.
- Remember the choice across page loads.

**Non-Goals:**
- No server-side sort, query parameter, or API change.
- No sort of the repository sidebar.
- No column headers / clickable sorting; a single select control is sufficient.

## Decisions

**Client-side sort in `web/app.js`.**
The grid is already a client-side render of a single project's worktrees. Sorting a small array in the browser is trivial, keeps the API contract untouched, and matches the existing "the UI holds presentation preferences in localStorage" pattern (see `gt_poll`, `gt_selected_project`).
*Alternative:* add a `?sort=` query parameter to the JSON API and sort in Go. Rejected — it couples the API to a UI preference, changes the shared contract for no benefit, and the data set per project is small.

**A `<select>` control with four options.**
Simplest control that expresses "field + direction" as a flat list. Options: `created_desc` (default), `created_asc`, `name_asc`, `name_desc`. Labels: "Newest created", "Oldest created", "Name (A→Z)", "Name (Z→A)".
*Alternative:* two controls (field + direction). More UI for no added capability here.

**Comparison keyed on `created_at` then `handle`, and on `handle` for name sort.**
`created_at` may be absent/0 for some worktrees; treat missing as oldest so it sorts to the end under newest-first. Ties and name sort use `handle` (case-insensitive `localeCompare`) for a stable, predictable order.
*Alternative:* sort by `branch` for name. Rejected — handle is the primary identifier shown on each card.

**Persist the choice in `localStorage` under a new key (e.g. `gt_sort`)** and re-apply it on init and after each render, mirroring how the poll interval and selected project are stored.

**Where the control lives.**
Add the select to the existing header controls area (next to poll/pause), so it is visible regardless of which project is selected. It affects only the grid, not the sidebar.

## Risks / Trade-offs

- [A worktree with missing `created_at` sorts unexpectedly] → Treat missing as the smallest possible value and fall back to `handle` for ties so order is deterministic.
- [Stale order after a new worktree appears] → Sorting is applied on every render/poll, so new worktrees land in the correct position automatically.
- [Extra control clutter in the header] → Acceptable; it is a single select and reuses existing control styling.
