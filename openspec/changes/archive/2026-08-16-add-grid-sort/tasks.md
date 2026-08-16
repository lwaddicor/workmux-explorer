## 1. Sort logic (web/app.js)

- [x] 1.1 Add a `sortWorktrees(worktrees, mode)` helper that returns a new array ordered per mode (`created_desc`, `created_asc`, `name_asc`, `name_desc`), treating a missing `created_at` as the oldest and using case-insensitive `handle` for name sort and ties.
- [x] 1.2 Apply `sortWorktrees` to `p.worktrees` inside `renderGrid` using the current sort mode, so every render/poll reflects the chosen order.
- [x] 1.3 Add a `getSortMode()` that reads `localStorage` key `gt_sort`, defaults to `created_desc` when unset or unrecognized.

## 2. UI control (web/index.html, web/style.css)

- [x] 2.1 Add a `<select id="sort-order">` with options "Newest created", "Oldest created", "Name (A→Z)", "Name (Z→A)" to the header controls area (next to poll/pause).
- [x] 2.2 Style the select to match the existing header controls in `web/style.css`.

## 3. Wiring (web/app.js)

- [x] 3.1 In `init`, set the select's value from `getSortMode()` and add a `change` listener that persists the choice to `localStorage` (`gt_sort`) and re-renders the grid via `render(lastInv)` without a page reload.

## 4. Verify

- [x] 4.1 Run `go build ./...`, `go vet ./...`, `gofmt -l .` (must be clean) and `go test ./...`.
- [x] 4.2 Manually start the dashboard on a worktree port (`go run ./cmd/gittreemux serve -listen 127.0.0.1:8788`) and confirm: default order is newest-created, each option reorders the grid correctly, the choice persists across reload, and the JSON API order is unchanged.
