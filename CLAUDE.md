# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the project (binary name: trelay)
make build

# Run directly
make run

# Run with debug logging
make run-debug

# Build and install to system
make install

# Uninstall from system
make uninstall

# Clean build artifacts
make clean

# Build cross-platform binaries
make build-all     # Linux, macOS (amd64/arm64), Windows
make build-linux   # Linux amd64
make build-darwin  # macOS amd64 and arm64
make build-windows # Windows amd64

# Run with specific config file
./trelay --config /path/to/config.json

# Direct SSH connection (bypasses TUI)
./trelay --direct-ssh "hostname"

# Direct SSH connection with password (bypasses TUI)
./trelay --direct-ssh "hostname" --password "password"

# Direct RDP connection (bypasses TUI)
./trelay --direct-rdp "hostname"

# Run with lint checking
go vet ./...

# Run tests (no tests exist yet)
go test ./...

# Format code
make fmt
```

## Architecture Overview

The application uses a protocol-agnostic design pattern:

```
cmd/rdm/main.go (CLI entry point)
    ├── Loads config via internal/config/
    ├── Handles direct connections (--direct-ssh, --direct-rdp)
    └── Spawns TUI via internal/ui/tui/

internal/protocol/ (Protocol abstraction layer)
    ├── session.go      - Session interface (Connect, Disconnect, etc.)
    ├── manager.go      - Connection manager
    ├── ssh/            - SSH implementation (golang.org/x/crypto/ssh)
    └── rdp/            - RDP implementation (external tools)

internal/ui/tui/ (Bubble Tea TUI)
    ├── app.go          - Main TUI application
    └── dialogs/        - Dialog components

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
- `internal/protocol/rdp/client.go` - Spawns external tools (Remmina/FreeRDP via exec.Cmd)

### Connection Flow

The TUI does not maintain connections directly. Instead, it uses `syscall.Exec` to replace the current process:

1. User selects host in TUI → `internal/ui/tui/app.go:executeConnection()`
2. TUI spawns new process with `--direct-ssh` or `--direct-rdp` flag
3. `cmd/rdm/main.go:runDirectConnection()` handles the connection
4. After disconnect, `--return-to-rdm` flag causes process restart to return to TUI

This avoids conflicts between Bubble Tea event loop and terminal control during SSH/RDP sessions.

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

- Default config path: `~/.config/remote-desktop-manager/config.json`
- Config auto-created if missing (see `internal/config/config.go:Load()`)
- Hot reload via `r` key in TUI
- Host grouping logic in `pkg/models/config.go:GetGroupedHosts()`

### Host Grouping

Hosts are organized into groups (plus "未分组" for ungrouped hosts):
- Groups defined in config contain lists of host names
- `GetGroupedHosts()` resolves names to host objects
- Tab key switches between groups in TUI

## Important File Locations

| File | Purpose |
|------|---------|
| `Makefile` | Build and deployment management |
| `cmd/rdm/main.go` | CLI entry, handles direct connections |
| `internal/ui/tui/app.go` | Bubble Tea TUI application |
| `internal/ui/dialogs/password.go` | SSH password input dialog |
| `internal/ui/dialogs/new_connection.go` | New connection configuration dialog |
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
4. Add protocol handling in `cmd/rdm/main.go:runDirectConnection()`
5. Update `internal/ui/tui/app.go:executeConnection()` for TUI
6. Update `internal/ui/tui/app.go:renderHostItem()` for protocol display

## Notes

- No test files exist yet (`go test ./...` will find nothing)
- Passwords stored in plaintext in config file (password prompt added for security)
- `InsecureIgnoreHostKey()` used for SSH - production should validate host keys
- macOS ioctl constants differ from Linux (see `app.go` constants)
- Terminal state is restored properly after TUI exit (sends ANSI escape sequences to restore terminal settings)
- TUI exit sends escape sequences to restore terminal state: exit alt screen, disable mouse tracking
- Password prompt dialog implemented for SSH connections without stored passwords
- All dialogs now use `lipgloss.Place` for perfect horizontal and vertical centering
