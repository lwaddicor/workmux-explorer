## 1. Layout shell

- [x] 1.1 Restructure `web/index.html`: replace `<main id="projects">` with a two-pane shell — `<nav id="sidebar">` and `<main id="worktree-grid">` inside a content wrapper
- [x] 1.2 Add two-pane layout styles to `web/style.css` (sidebar fixed width, grid pane fills the rest) and keep the header, banner, modals, and detail panel working

## 2. Sidebar rendering

- [x] 2.1 In `web/app.js`, split `render()` into `renderSidebar(inv)` and `renderGrid(inv)`: sidebar lists every project with name, worktree count, and active-agent count
- [x] 2.2 Add selection state: `localStorage` key `gt_selected_project`, restore on load with fallback to the first project when the saved name is missing
- [x] 2.3 Wire sidebar item clicks to change the selection (persist it) and re-render the grid; mark the selected item with `aria-current`

## 3. Grid rendering

- [x] 3.1 Render the selected project's worktrees into `#worktree-grid` as a responsive card grid, reusing the existing `renderWorktree()` markup unchanged
- [x] 3.2 Move the existing `button[data-act]` click delegation onto the grid container so open/close/send/output/remove keep working
- [x] 3.3 Handle empty states: no projects → existing full-page empty message; selected project with zero worktrees → inline "No worktrees" message in the grid pane
- [x] 3.4 Keep the status badge, degraded banner, and polling controls behaving as before

## 4. Responsive and polish

- [x] 4.1 Add a narrow-viewport media query: sidebar becomes a horizontally scrollable row of repository chips above the grid
- [x] 4.2 Verify all worktree actions (open window, close window, send prompt, output panel, remove with confirm) work in the new layout

## 5. Verification

- [x] 5.1 Run `go build ./...` and `go vet ./...` to confirm the embedded assets still compile
- [x] 5.2 Manually test in a browser: switch repositories via the sidebar, confirm the grid shows only that repository's worktrees, confirm polling updates statuses without a reload, confirm selection survives a page reload
