# Contributing to siding

## Setup

Requires Go (version in `go.mod`) and git.

```
git clone https://github.com/rans0/siding && cd siding
go build ./cmd/siding            # binary in the current dir
go run ./cmd/siding version      # or run without building
```

To try it end to end, build the binary, put it on your `PATH`, and use a scratch workspace: `siding init /tmp/ws`.

## Layout

| Path | What |
| --- | --- |
| `cmd/siding` | CLI: flag parsing and output only |
| `internal/core` | Everything else: workspace, tracks, stages, adapters, guard, learn, doctor, PTY sessions |
| `internal/ui` | Board: HTTP handlers and the terminal websocket |
| `web/` | Board templates and CSS; `web/xterm/` is vendored xterm.js |
| `templates/` | `AGENTS.md.tmpl`, default `siding.toml`, task doc templates |
| `starter-knowledge/` | Rules and skills copied into a new workspace |
| `embed.go` | Embeds `web`, `templates` and `starter-knowledge` into the binary |

Logic goes in `internal/core` so the CLI and the board share it. `cmd/siding` and `internal/ui` stay thin.

## Principles

- Git is the only hard dependency. Other tools (`gh`, `node`, an agent) are optional or checked at use.
- Plain files: TOML config, Markdown docs, JSON track state. No database, no daemon.
- Stdlib first. Adding a Go module needs a reason a few lines of code can't cover.
- The board keeps no state; it calls the same code as the CLI.

## Tests

```
go vet ./...
go test ./...
```

Tests use real git repos in `t.TempDir()`, with no network. Add one for each behavior you change.

CI runs on Linux, macOS and Windows. Before pushing, check the other platforms compile:

```
GOOS=windows go vet ./... && GOOS=linux go vet ./...
```

Windows pitfalls:

- Build paths with `filepath.Join`, never with `/` in a string, including in test expectations.
- Build JSON with `json.Marshal`. A Windows path pasted into a JSON string breaks on its backslashes.
- Shell commands go through `shell()` (`sh -c` or `cmd /c`); don't call `sh` directly.
- Unix-only code goes in `*_unix.go` with a `*_windows.go` counterpart, or a `//go:build !windows` test.

PTY sessions have no PTY on Windows: `siding open` runs the agent in the foreground there, and `session_test.go` is skipped.

## Adapters

An adapter writes harness files into each track (`internal/core/adapter.go`). A new one needs:

1. An entry in the `adapters` map.
2. Its hook payload handled by `Guard`, with a case in `TestGuardPayloads`.
3. Its files checked in `TestAdapterFiles`.
4. A row in the README's harness table.

Check hook payload formats against the harness's current docs; they change.

## Commits and PRs

- One change per commit, message in the imperative ("Add X", "Fix Y").
- Update `README.md` when a command, flag or behavior changes.
- `docs/` holds local design notes and is not committed.

## Releases

Push a tag `vX.Y.Z`. The release workflow runs the tests, then GoReleaser builds binaries for Linux, macOS and Windows and updates the Homebrew tap and Scoop bucket. It needs a `TAP_TOKEN` secret that can push to `rans0/homebrew-tap` and `rans0/scoop-bucket`.

## Vendored xterm.js

`web/xterm/` holds `xterm.js`, `xterm.css` and `LICENSE` from the `@xterm/xterm` npm package. To upgrade, replace all three from the new version and check the board terminal in a browser.
