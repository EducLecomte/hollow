# AGENTS.md

Hollow is a TUI file explorer + text editor in Go (tview/tcell) supporting local, FTP, FTPS, SFTP, and archives (`.zip`, `.tar`, `.tar.gz`). Code comments and all user-facing strings are in French — keep that convention.

## Commands

- Build: `make build` → writes `./hollow` in the repo root, injecting `main.Version` via `-ldflags` from `git describe --tags`.
- `hollow` and `bin/hollow` are committed build artifacts (there is no `.gitignore`) — do not commit rebuild output.
- Test: `go test ./...` (fast, no network, no TTY — tests never call `App.Run()`).
  - Single test: `go test ./internal/app/ -run TestPanelStateBasics`
- No linter/formatter config and no CI test job; `go vet ./...` is clean. Go 1.26 (`go.mod`).
- Release: pushing a `v*` tag triggers CI to build linux `amd64`/`arm64` as `hollow-linux-<arch>` and publish a GitHub Release. `install.sh` / `install-alpine.sh` download the latest release.

## Architecture

- `cmd/hollow/main.go` — entrypoint; optional positional arg is an initial directory or file; a `recover()` wrapper restores the terminal on panic.
- `internal/app/` — all tview UI. `EditorApp` (ui.go) owns two `PanelState`s; `rebuildMainLayout()` regenerates the whole layout: default mode = explorer + viewer, dual-pane only when a remote is connected or transfer mode (F6) is on.
- `internal/vfs/` — the `VFS` interface isolates protocols: `LocalFS`, `FtpFS` (FTP/FTPS), `SftpFS`, `ArchiveFS`. `ArchiveFS` builds an in-memory tree and is read-only: all mutating ops return an error.
- `internal/utils/` — helpers (chroma syntax highlighting for the viewer, binary detection, help texts).

## Gotchas

- Any tview call from a goroutine (async previews, status updates, remote loads) must go through `e.App.QueueUpdateDraw(...)` — the codebase is consistent about it.
- In each panel's list, index 0 is always the `".."` entry; item `i` maps to `CurrentFiles[i-1]`.
- Remote connections are always attached to `RightPanel`; disconnect restores `PreviousFS`/`PreviousDir`.
- On exit the app writes the last local directory to `/tmp/hollow_cwd_$USER` (`saveLastDir`); the shell wrapper installed by `install.sh` `cd`s into it — preserve this contract.
- Favorites persist to `os.UserConfigDir()/hollow/favorites.json`; entries 0 and 1 are always forced to Home and `/`.
- The TUI needs a TTY; it cannot be run interactively in headless/CI environments — verify with `go build`, `go vet`, and the unit tests instead.
