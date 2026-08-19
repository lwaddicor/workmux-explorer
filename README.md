# workmux-explorer

A local web dashboard for [workmux](https://github.com/lwaddicor/workmux) worktrees. A single, self-contained Go binary that surfaces every workmux worktree and running agent on your machine, and lets you act on them from the browser.

## Features

- **Cross-project discovery** — enumerates all workmux worktrees across every project on the machine in one view.
- **Live agent output** — capture recent terminal output from any running agent.
- **Actions from the browser**:
  - **Open / close** the worktree's tmux window.
  - **Focus** a worktree — switches to its tmux window and brings the hosting terminal to the foreground.
  - **Send** a prompt to a running agent.
  - **Remove** a worktree (with explicit confirmation).
- **Loopback-only** — binds to `127.0.0.1` by default. No public exposure, no authentication to manage.
- **Zero dependencies** — Go standard library only. No framework, no build step, no external runtime.

## Requirements

- Go 1.26+
- [workmux](https://github.com/lwaddicor/workmux) installed and on your `PATH`
- `tmux`

## Install

```sh
go install github.com/lwaddicor/workmux-explorer@latest
```

Or build from source:

```sh
git clone https://github.com/lwaddicor/workmux-explorer
cd workmux-explorer
go build -o workmux-explorer ./cmd/workmux-explorer
```

## Usage

```sh
workmux-explorer serve
```

Then open <http://127.0.0.1:8787> in your browser.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `127.0.0.1:8787` | Host:port to bind |
| `-prefix` | `wm-` | Tmux window name prefix used for discovery |
| `-concurrency` | `8` | Max concurrent project reads |
| `-cache-ttl` | `2s` | Per-project result cache TTL |
| `-log` | *(stderr)* | Optional action log file path |
| `-workmux` | `workmux` | Path to the workmux binary |

### Version

```sh
workmux-explorer version
```

## API

The dashboard exposes a small JSON API alongside the embedded web UI.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects` | Full cross-project inventory |
| `GET` | `/api/projects/{project}/worktrees/{handle}` | A single worktree record |
| `GET` | `/api/projects/{project}/worktrees/{handle}/output` | Recent agent output |
| `POST` | `/api/projects/{project}/worktrees/{handle}/open` | Open the tmux window |
| `POST` | `/api/projects/{project}/worktrees/{handle}/close` | Close the tmux window |
| `POST` | `/api/projects/{project}/worktrees/{handle}/focus` | Focus the tmux window + terminal |
| `POST` | `/api/projects/{project}/worktrees/{handle}/send` | Send a prompt to the agent (`{"text": "…"}`) |
| `POST` | `/api/projects/{project}/worktrees/{handle}/remove` | Remove worktree (`{"confirmed": true}`) |
| `GET` | `/api/health` | Liveness probe |

## Development

```sh
go build ./...   # build
go test ./...    # test
go vet ./...     # vet
gofmt -l .       # must return nothing
```

Run a dev instance on a different port (the main worktree owns 8787):

```sh
go run ./cmd/workmux-explorer serve -listen 127.0.0.1:8788
```

## License

[Apache 2.0](LICENSE)
