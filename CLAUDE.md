# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the project (binary name: trelay, CGO_ENABLED=0, stripped symbols)
make build

# Run directly
make run

# Run with debug logging
make run-debug

# Build and install to /usr/local/bin
make install

# Clean build artifacts
make clean

# Build cross-platform binaries
make build-all     # Linux, macOS (amd64/arm64)
make build-linux   # Linux amd64 and arm64
make build-darwin  # macOS amd64 and arm64
make build-windows # Windows amd64

# Lint and format
make vet           # go vet ./...
make fmt           # gofmt -w on all Go sources

# Run tests (no tests exist yet)
make test          # go test -v ./...

# Run with specific config file
./trelay --config /path/to/config.json

# Direct SSH connection (bypasses TUI)
./trelay --direct-ssh "hostname"
./trelay --direct-ssh "hostname" --password "password"

# Direct RDP/VNC connection (bypasses TUI)
./trelay --direct-rdp "hostname"
./trelay --direct-vnc "hostname"
```

## Architecture Overview

```
cmd/trelay/main.go (CLI entry point)
    ├── Loads config via internal/config/
    ├── Handles direct connections (--direct-ssh, --direct-rdp, --direct-vnc)
    └── Spawns TUI via internal/ui/tui/

internal/protocol/ (Protocol abstraction layer)
    ├── session.go      - Session interface (Connect, Disconnect, etc.)
    ├── manager.go      - Connection manager (tracks background SSH sessions)
    ├── ssh/            - SSH implementation (golang.org/x/crypto/ssh)
    │   ├── client.go       - SSH client, connection, interactive session
    │   ├── pty_session.go  - PTY session with Attach/Detach for background support
    │   └── exec_adapter.go - Adapts PTYSession to Bubble Tea's tea.ExecCommand
    ├── rdp/            - RDP implementation (external tools)
    └── vnc/            - VNC implementation (external tools)

internal/ui/tui/ (Bubble Tea TUI)
    ├── app.go          - Main TUI application
    └── dialogs/        - Dialog components (new/edit connection, password, group, error)

pkg/models/ (Data models)
    ├── host.go         - Host configuration + validation
    ├── config.go       - Config structure + grouping logic
    └── connection.go   - Connection model
```

## Key Architecture Patterns

### Dual Connection Architecture: In-Process vs Process Replacement

This is the most important architectural distinction. SSH and RDP/VNC use fundamentally different connection strategies:

| Aspect | SSH | RDP / VNC |
|--------|-----|-----------|
| Connection method | In-process via `tea.Exec()` | `syscall.Exec` process replacement |
| Background support | Yes (`Ctrl+B` to detach) | No |
| Return to TUI | Bubble Tea resumes naturally | Re-exec binary with `--return-to-trelay` |
| Session tracking | Managed by `protocol.Manager` | Not tracked |
| Terminal restoration | `term.Restore()` in ExecAdapter | `stty sane` in `main.go:restoreTerminal()` |

### SSH In-Process PTY Architecture (CRITICAL)

SSH uses an in-process PTY model with background session support. The flow through Bubble Tea:

1. User presses Enter → `executeSSHConnection()` returns async `tea.Cmd`
2. Async: creates `Client`, calls `Connect()`, then `StartBackgroundSession()` which creates `PTYSession` and **transfers SSH connection ownership** from Client to PTYSession (`c.client = nil`)
3. `sshConnectResultMsg` arrives → session stored in `connManager` → `attachSSHSession()` creates `ExecAdapter` → `tea.Exec(adapter, callback)` releases terminal to adapter
4. Adapter sets raw mode, runs `PTYSession.Attach()` which blocks until Ctrl+B or session end
5. `sshSessionMsg` arrives → if `IsAlive()`, session was backgrounded; otherwise cleaned up

### PTYSession Persistent Output Channel Pattern (CRITICAL)

`pty_session.go` uses a **persistent `outputCh` channel** pattern to avoid goroutine leaks on Attach/Detach cycles:

- Two goroutines started in `Start()` read from `stdoutPipe`/`stderrPipe` for the **entire session lifetime**, sending data to a shared `outputCh chan []byte` (buffered, size 256)
- `Attach()` creates a consumer goroutine that reads from `outputCh` and writes to terminal stdout
- `Detach()` cancels the consumer; the persistent readers keep running (data buffered in channel, backpressure via SSH flow control when full)

**NEVER add additional goroutines that read from `stdoutPipe`/`stderrPipe` directly** — this would race with the persistent readers and cause data loss. All SSH output must flow through `outputCh`.

### SSH Detach/Attach Mechanism

- **Ctrl+B (0x02)** is the detach hotkey, detected in the stdin forwarding goroutine
- `cancelreader` (from `github.com/muesli/cancelreader`) wraps stdin to allow cancelling blocked reads
- `pendingInput` preserves bytes read after Ctrl+B in the same buffer, replayed on next Attach
- `ExecAdapter.Run()` intentionally uses `os.Stdin`/`os.Stdout` directly, **not** the Bubble Tea-provided stdin/stdout (because Bubble Tea's cancelreader gets cancelled on `ReleaseTerminal`)

### SIGWINCH Terminal Resize Handling

Terminal resize is handled in **two separate places** for two code paths:

1. `exec_adapter.go:Run()` — TUI path, calls `PTYSession.ResizeTerminal()`
2. `client.go:StartInteractiveSession()` — `--direct-ssh` path, calls `session.WindowChange()` directly

Both use `getTermSize()` in `client.go` which calls `TIOCGWINSZ` ioctl with platform-specific constants.

### RDP/VNC: Process Replacement Path

RDP and VNC use `syscall.Exec` to replace the process with the connection binary:

1. TUI calls `executeExternalConnection()` which execs with `--direct-rdp`/`--direct-vnc` + `--return-to-trelay` flags
2. `main.go:runDirectConnection()` handles the connection
3. After disconnect, `--return-to-trelay` causes re-exec back to TUI
4. `restoreTerminal()` (stty sane + ANSI reset sequences) is called to fix terminal state

RDP tool detection chain: `detector.go` → `selector.go` (platform priority) → `builder.go` (command factory)
- **Linux**: Remmina (GUI) → FreeRDP (CLI fallback)
- **macOS**: FreeRDP only

VNC tool detection:
- **Linux**: Remmina → TigerVNC
- **macOS**: Built-in Screen Sharing via `open vnc://` URL scheme

**Note**: VNC client defines its own `ConnectionStatus` type and does NOT satisfy the `protocol.Session` interface. It only works through the process replacement path.

### Configuration Management

- Default config path: `~/.config/trelay/config.json`
- Config auto-created if missing (see `internal/config/config.go:Load()`)
- `r` key in TUI: reloads config + triggers async host status check
- Host grouping logic in `pkg/models/config.go:GetGroupedHosts()`
- Tab key switches between groups; "未分组" for ungrouped hosts

### Host Status Monitoring

- `checkHostStatusAsync()` performs async TCP dial (2s timeout) to all hosts to determine online/offline status
- Runs automatically every 3 seconds via `statusCheckCmd()` (uses `tea.Every`)
- Also triggered on startup and when pressing `r`

## Important File Locations

| File | Purpose |
|------|---------|
| `cmd/trelay/main.go` | CLI entry, direct connections, terminal restoration |
| `internal/ui/tui/app.go` | Bubble Tea TUI (no alt screen mode) |
| `internal/protocol/ssh/pty_session.go` | SSH PTY session with Attach/Detach and persistent outputCh |
| `internal/protocol/ssh/exec_adapter.go` | Adapts PTYSession to tea.ExecCommand, handles SIGWINCH |
| `internal/protocol/ssh/client.go` | SSH client, getTermSize(), interactive session with SIGWINCH |
| `internal/protocol/rdp/detector.go` | RDP tool detection logic |
| `pkg/models/host.go` | Host model with `Validate()` and `GetPort()` |
| `pkg/models/config.go` | Config model with grouping logic |

## Adding a New Protocol

1. Create `internal/protocol/<protocol>/client.go` implementing `Session` interface
2. Add protocol-specific fields to `pkg/models/host.go`
3. Update `pkg/models/host.go:DefaultPort()` and `Validate()`
4. Add protocol handling in `cmd/trelay/main.go:runDirectConnection()`
5. Update `internal/ui/tui/app.go:executeConnection()` for TUI
6. Update `internal/ui/tui/app.go:renderHostItem()` for protocol display

## Notes

- No test files exist yet (`go test ./...` will find nothing)
- Passwords stored in plaintext in config file
- `InsecureIgnoreHostKey()` used for SSH — production should validate host keys
- **TUI does NOT use alt screen mode** (see `app.go:Run()`) — helps with terminal state issues
- `EditConnectionDialog` and `NewConnectionDialog` share ~80% identical code — keep them in sync when modifying

### Platform-Specific Constants

macOS and Linux use different ioctl constants:

- **TIOCGWINSZ** (get terminal size): macOS `0x40087468`, Linux `0x5413` — used in `internal/protocol/ssh/client.go`
- **TCGETS/TCSETS** (get/set terminal attributes): macOS `0x40487413`/`0x80487414`, Linux `0x5401`/`0x5402` — used in `internal/ui/tui/app.go`

When adding new terminal ioctl calls, always handle both platforms.

### TUI Key Bindings (for understanding Update() logic in app.go)

Normal mode: `q`/`Ctrl+C` quit, `/` search, `↑`/`↓`/`j`/`k` navigate, `Enter` connect, `Tab` switch group, `N` new connection, `G` new group, `E` edit connection, `r` reload config + refresh status, `D`/`Delete` delete connection, `B` background session list

Search mode: character input, `Backspace` delete, `←`/`→` cursor, `Esc` exit search

SSH session: `Ctrl+B` detach to background
