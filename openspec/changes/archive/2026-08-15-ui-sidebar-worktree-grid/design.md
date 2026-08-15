## Context

The dashboard is a framework-free single page (vanilla JS, no build step) served from an embedded static directory (`web/index.html`, `web/app.js`, `web/style.css`, embedded via `web/embed.go`). The spec requires a single self-contained binary with no frontend build step, so the change stays in plain HTML/CSS/JS.

Current rendering (`web/app.js` `render()`/`renderProject()`/`renderWorktree()`) builds one long vertical list: each project is a `<section>` with its worktrees as a stacked column of `<article>` cards. The inventory comes from `GET /api/projects` as `{degraded, projects: [{name, root, error, worktrees: [...]}]}` and is re-fetched on a polling interval.

## Goals / Non-Goals

**Goals:**
- Two-pane layout: repository sidebar (left) + worktree grid (right).
- Selection state that survives polling re-renders and page reloads.
- Grid of worktree cards that reflows with viewport width.
- Keep all existing behavior: polling controls, degraded banner, worktree actions, send-prompt modal, output detail panel.

**Non-Goals:**
- No API or backend changes.
- No framework, bundler, or build step.
- No repository search/filter, sorting, or keyboard navigation.

## Decisions

1. **Selection is client-side state, not a route.** The selected repository is a JS variable persisted to `localStorage` (key `gt_selected_project`). On load, restore it if it still exists in the inventory; otherwise default to the first project. Alternatives considered: a per-project URL (query param or path) would require either backend route changes or history-API handling for no user-visible benefit in a local single-user dashboard.

2. **Keep the existing data flow; split rendering into two functions.** `render()` becomes: render the sidebar (all projects with name + "N worktrees · M active" summary), then render the grid for the selected project only. The existing `renderWorktree()` card markup is reused unchanged, so all `data-act` buttons and the existing `onWorktreeClick` delegation on the grid container keep working as-is. Alternatives considered: reusing one full-page re-render per poll but keeping the list DOM — rejected because the spec delta requires a grid of the selected repo's worktrees.

3. **Layout via CSS grid, no structural changes to modals.** Replace `<main id="projects">` with a flex/grid shell: `<nav id="sidebar">` + `<main id="worktree-grid">`. The detail panel and send modal stay as fixed overlays. The sidebar item is a button with `aria-current` on the selected one.

4. **Responsive fallback.** Below a narrow-viewport breakpoint the sidebar becomes a horizontally scrollable row of repository chips above the grid, instead of a vertical column. One media query, no JS change.

5. **Empty and error states.** No projects → existing full-page empty message. Selected project with zero worktrees → a small "No worktrees" message inside the grid pane (sidebar stays visible so the user can switch repositories).

## Risks / Trade-offs

- [Polling re-render every ~4s can drop transient UI state (e.g. hover, focus)] → same behavior already exists today for the full list; buttons and modals are the only interactive state and are unaffected. If needed, re-rendering can be diffed later; not required.
- [Selection persisted in localStorage may point at a renamed/removed repository] → fall back to the first project when the saved name is absent from the inventory.
- [Many repositories could make the sidebar long] → acceptable for a local dashboard; the chip fallback keeps it navigable on narrow screens.

## Migration Plan

Static assets are embedded in the binary; deploying the new binary replaces the UI. Rollback is redeploying the previous binary. No data or state migration (the new `localStorage` key is additive).
