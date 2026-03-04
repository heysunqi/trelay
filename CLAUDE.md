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

# Uninstall from system
make uninstall

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

# Direct SSH connection with password (bypasses TUI)
./trelay --direct-ssh "hostname" --password "password"

# Direct RDP connection (bypasses TUI)
./trelay --direct-rdp "hostname"
```

## Architecture Overview

The application uses a protocol-agnostic design pattern:

```
cmd/trelay/main.go (CLI entry point)
    ├── Loads config via internal/config/
    ├── Handles direct connections (--direct-ssh, --direct-rdp)
    └── Spawns TUI via internal/ui/tui/

internal/protocol/ (Protocol abstraction layer)
    ├── session.go      - Session interface (Connect, Disconnect, etc.)
    ├── manager.go      - Connection manager
    ├── ssh/            - SSH implementation (golang.org/x/crypto/ssh)
    └── rdp/            - RDP implementation (external tools)

internal/ui/tui/ (Bubble Tea TUI)
    ├── app.go          - Main TUI application (~1400 lines)
    └── dialogs/        - Dialog components
        ├── new_connection.go  - New connection form
        ├── edit_connection.go - Edit connection form (shares ~80% code with new_connection)
        ├── password.go        - SSH password input
        ├── new_group.go       - Group creation
        └── error.go           - Error display

pkg/models/ (Data models)
    ├── host.go         - Host configuration + validation
    ├── config.go       - Config structure + grouping logic
    └── connection.go   - Connection model
```

## Key Architecture Patterns

### Protocol Abstraction

All protocols implement the `Session` interface defined in `internal/protocol/session.go`:

```go
type Session interface {
    Connect() error
    Disconnect() error
    IsConnected() bool
    GetStatus() ConnectionStatus
    GetError() error
    GetHostID() string
    GetStartTime() *time.Time
    GetDuration() time.Duration
}
```

- `internal/protocol/ssh/client.go` - Uses `golang.org/x/crypto/ssh` library
- `internal/protocol/rdp/client.go` - Spawns external tools (Remmina/FreeRDP) via exec.Cmd

### Connection Flow

The TUI does not maintain connections directly. Instead, it uses `syscall.Exec` to replace the current process:

1. User selects host in TUI → `internal/ui/tui/app.go:executeConnection()`
2. TUI execs current binary with `--direct-ssh` or `--direct-rdp` + `--return-to-trelay` flags
3. `cmd/trelay/main.go:runDirectConnection()` handles the connection
4. After disconnect, `--return-to-trelay` flag causes the binary to re-exec itself (without direct flags) to return to TUI
5. **`restoreTerminal()`** is called after SSH/RDP session ends to fix terminal state

This avoids conflicts between Bubble Tea event loop and terminal control during SSH/RDP sessions.

### Terminal State Restoration (CRITICAL for SSH sessions)

SSH sessions change terminal settings (raw mode, hidden cursor, etc.). After SSH exits:

- `cmd/trelay/main.go:restoreTerminal()` is called
- Uses `stty sane` command to reset terminal to default state
- Sends ANSI escape sequences as fallback: `\033[?1049l` (exit alt screen), `\033[?25h` (show cursor), `\033[0m` (reset attributes)
- Called in three places:
  1. After successful direct SSH/RDP connection ends (before returning to TUI)
  2. After failed direct connection (before showing error TUI)
  3. After TUI exits (before program termination)

If you encounter terminal corruption after SSH sessions, this function needs fixing.

### RDP Tool Detection (internal/protocol/rdp/)

RDP uses external tools detected at runtime:

**Linux**: Remmina (GUI) → FreeRDP (CLI fallback)
**macOS**: FreeRDP only

The detection chain:
- `types.go` - Tool type definitions and capabilities
- `selector.go` - Platform-specific tool priority
- `detector.go` - `exec.LookPath()` to find available tools
- `install_helper.go` - Detects package managers (apt/yum/dnf/pacman/apk) and generates install commands
- `builder.go` - Command builder factory
- `remmina_builder.go` - Builds `remmina --connect=rdp://...` commands
- `freerdp_builder.go` - Builds `xfreerdp /v:... /u:... /p:...` commands with dynamic resolution support

If no tool is found, the error includes platform-specific install help.

### RDP Features

- **Dynamic Resolution Adjustment**: Remote desktop resolution automatically adjusts to window size changes
- **Platform-Specific Error Messages**: Friendly error messages with platform-specific solutions
  - macOS: Instructions for installing XQuartz
  - Linux: Instructions for X server setup
  - Both: SSH X11 forwarding option

### Configuration Management

- Default config path: `~/.config/trelay/config.json`
- Config auto-created if missing (see `internal/config/config.go:Load()`)
- Hot reload via `r` key in TUI
- Host grouping logic in `pkg/models/config.go:GetGroupedHosts()`

### Host Grouping

Hosts are organized into groups (plus "未分组" for ungrouped hosts):
- Groups defined in config contain lists of host names
- `GetGroupedHosts()` resolves names: to host objects
- Tab key switches between groups in TUI

## Important File Locations

| File | Purpose |
|------|---------|
| `Makefile` | Build and deployment management |
| `cmd/trelay/main.go` | CLI entry, handles direct connections, terminal restoration |
| `internal/ui/tui/app.go` | Bubble Tea TUI application (no alt screen mode) |
| `internal/ui/dialogs/password.go` | SSH password input dialog |
| `internal/ui/dialogs/new_connection.go` | New connection configuration dialog |
| `internal/ui/dialogs/edit_connection.go` | Edit connection dialog (largely mirrors new_connection.go) |
| `internal/ui/dialogs/new_group.go` | New group creation dialog |
| `internal/ui/dialogs/error.go` | Error display dialog with text wrapping |
| `internal/protocol/session.go` | Session interface definition |
| `internal/protocol/ssh/client.go` | SSH implementation |
| `internal/protocol/rdp/client.go` | RDP implementation |
| `internal/protocol/rdp/detector.go` | RDP tool detection logic |
| `internal/protocol/rdp/freerdp_builder.go` | FreeRDP command builder |
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
- Passwords stored in plaintext in config file (password prompt added for security)
- `InsecureIgnoreHostKey()` used for SSH - production should validate host keys
- **TUI does NOT use alt screen mode** (see `app.go:Run()`) - this helps with terminal state issues
- Terminal state restoration uses `stty sane` command - most reliable way to fix terminal after SSH sessions
- All dialogs use `lipgloss.Place` for perfect horizontal and vertical centering
- `EditConnectionDialog` and `NewConnectionDialog` share ~80% identical code (field definitions, visibility logic, navigation, rendering, validation) - keep them in sync when modifying

### Platform-Specific Constants

macOS and Linux use different ioctl constants. These are defined with build-tag-like conditional logic:

- **TIOCGWINSZ** (get terminal size): macOS `0x40087468`, Linux `0x5413` — used in `internal/protocol/ssh/client.go`
- **TCGETS/TCSETS** (get/set terminal attributes): macOS `0x40487413`/`0x80487414`, Linux `0x5401`/`0x5402` — used in `internal/ui/tui/app.go`

When adding new terminal ioctl calls, always handle both platforms.

### TUI Key Bindings (for understanding Update() logic in app.go)

Normal mode: `q`/`Ctrl+C` quit, `/` search, `↑`/`↓`/`j`/`k` navigate, `Enter` connect, `Tab` switch group, `N` new connection, `G` new group, `E` edit connection, `r` reload config, `D`/`Delete` delete connection

Search mode: character input, `Backspace` delete, `←`/`→` cursor, `Esc` exit search
