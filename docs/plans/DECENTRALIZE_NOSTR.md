# ORLY Expansion Plan: Documentation, Installer, Tray, and WireGuard

## Overview

Expand ORLY from a relay binary into a complete ecosystem for personal Nostr relay deployment, with:
1. **Textbook-style README** - Progressive documentation from novice to expert
2. **GUI Installer** - Wails-based setup wizard (Linux + macOS)
3. **System Tray** - Service monitoring and control
4. **WireGuard Client** - Embedded tunnel for NAT traversal
5. **Proxy Server** - Self-hostable AND managed service option

---

## Architecture

```
                              USER SYSTEMS
┌─────────────────────────────────────────────────────────────────────┐
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │ orly-setup   │    │    orly      │    │ orly --tray  │          │
│  │ (Installer)  │    │   (Relay)    │    │ (Systray)    │          │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘          │
│         │                   │                   │                   │
│         │ generates         │ serves            │ monitors          │
│         ▼                   ▼                   ▼                   │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │ ~/.config/   │    │ :3334 WS/HTTP│    │ /api/admin/* │          │
│  │ systemd svc  │    │ + WG tunnel  │    │ status/ctrl  │          │
│  └──────────────┘    └──────┬───────┘    └──────────────┘          │
│                             │                                       │
│                     ┌───────┴───────┐                              │
│                     │  pkg/tunnel/  │                              │
│                     │  WireGuard    │                              │
│                     └───────┬───────┘                              │
└─────────────────────────────┼───────────────────────────────────────┘
                              │ WG Tunnel (UDP :51820)
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        PROXY SERVER                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │ WG Server    │───▶│ Nostr Auth   │───▶│ Public Proxy │          │
│  │ :51820       │    │ (npub-based) │    │ Egress       │          │
│  └──────────────┘    └──────────────┘    └──────────────┘          │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Package Structure

```
next.orly.dev/
├── cmd/
│   ├── orly-setup/              # NEW: Wails installer
│   │   ├── main.go
│   │   ├── app.go               # Backend logic
│   │   ├── frontend/            # Svelte wizard UI
│   │   │   └── src/steps/       # Welcome, Config, Install, Complete
│   │   └── install/
│   │       ├── preflight.go     # Dependency checks
│   │       ├── systemd.go       # Service creation
│   │       └── verify.go        # Post-install checks
│   │
│   └── proxy-server/            # NEW: WireGuard proxy
│       ├── main.go
│       ├── server.go            # WG server
│       ├── auth.go              # Nostr auth
│       └── registry.go          # User management
│
├── pkg/
│   ├── tunnel/                  # NEW: Embedded WG client
│   │   ├── tunnel.go            # Main interface
│   │   ├── client.go            # wireguard-go wrapper
│   │   ├── reconnect.go         # Auto-reconnect
│   │   └── health.go            # Connection health
│   │
│   ├── tray/                    # NEW: System tray
│   │   ├── tray.go              # Platform abstraction
│   │   ├── tray_linux.go        # Linux implementation
│   │   ├── tray_darwin.go       # macOS implementation
│   │   └── menu.go              # Menu construction
│   │
│   ├── admin/                   # NEW: Admin HTTP API
│   │   ├── api.go               # Router
│   │   ├── status.go            # GET /api/admin/status
│   │   ├── control.go           # POST /api/admin/start|stop|restart
│   │   └── logs.go              # GET /api/admin/logs (SSE)
│   │
│   └── interfaces/
│       ├── tunnel/tunnel.go     # Tunnel interface
│       ├── tray/tray.go         # Tray interface
│       └── admin/admin.go       # Admin API interface
│
└── docs/
    └── README.adoc              # NEW: Textbook-style docs
```

---

## Implementation Phases

### Phase 1: Documentation Foundation
**Files to create/modify:**
- `README.adoc` - New textbook-style documentation
- `docs/` - Reorganize scattered docs

**README Structure (Textbook Style):**
```
Chapter 1: Quick Start (5-minute setup)
Chapter 2: Installation (platform-specific)
Chapter 3: Configuration (all env vars)
Chapter 4: Operations (systemd, monitoring)
Chapter 5: Security (TLS, ACLs, policy)
Chapter 6: Advanced (Neo4j, clustering, WoT)
Chapter 7: Architecture (internals)
Appendices: Reference tables, troubleshooting
```

### Phase 2: Admin API
**Files to create:**
- `pkg/admin/api.go` - Router and middleware
- `pkg/admin/status.go` - Status endpoint
- `pkg/admin/control.go` - Start/stop/restart
- `pkg/admin/logs.go` - Log streaming via SSE
- `pkg/interfaces/admin/admin.go` - Interface definition

**Files to modify:**
- `app/server.go` - Register `/api/admin/*` routes
- `app/config/config.go` - Add admin API config

**Endpoints:**
```
GET  /api/admin/status   - Relay status, uptime, connections
POST /api/admin/start    - Start relay (when in tray mode)
POST /api/admin/stop     - Graceful shutdown
POST /api/admin/restart  - Graceful restart
GET  /api/admin/logs     - SSE log stream
```

### Phase 3: System Tray
**Files to create:**
- `pkg/tray/tray.go` - Platform abstraction
- `pkg/tray/tray_linux.go` - Linux (dbus/appindicator)
- `pkg/tray/tray_darwin.go` - macOS (NSStatusBar)
- `pkg/tray/menu.go` - Menu construction
- `pkg/interfaces/tray/tray.go` - Interface

**Files to modify:**
- `main.go` - Add `--tray` flag handling
- `app/config/config.go` - Add tray config

**Features:**
- Status icon (green/yellow/red)
- Start/Stop/Restart menu items
- Open Web UI (launches browser)
- View Logs submenu
- Auto-start on login toggle

### Phase 4: Installer GUI (Wails)
**Files to create:**
- `cmd/orly-setup/main.go` - Wails entry point
- `cmd/orly-setup/app.go` - Backend methods
- `cmd/orly-setup/frontend/` - Svelte wizard
- `cmd/orly-setup/install/preflight.go` - Dependency checks
- `cmd/orly-setup/install/systemd.go` - Service creation
- `cmd/orly-setup/install/config.go` - Config generation
- `cmd/orly-setup/install/verify.go` - Post-install checks
- `scripts/build-installer.sh` - Build script

**Wizard Steps:**
1. Welcome - Introduction, license
2. Preflight - Check Go, disk, ports
3. Configuration - Port, data dir, TLS domains
4. Admin Setup - Generate or import admin keys
5. Database - Choose Badger or Neo4j
6. WireGuard (optional) - Tunnel config
7. Installation - Create service, start relay
8. Complete - Verify and show status

### Phase 5: WireGuard Client
**Files to create:**
- `pkg/tunnel/tunnel.go` - Main interface
- `pkg/tunnel/client.go` - wireguard-go wrapper
- `pkg/tunnel/config.go` - WG configuration
- `pkg/tunnel/reconnect.go` - Auto-reconnect logic
- `pkg/tunnel/health.go` - Health monitoring
- `pkg/tunnel/handoff.go` - Graceful restart
- `pkg/interfaces/tunnel/tunnel.go` - Interface

**Files to modify:**
- `app/config/config.go` - Add WG config fields
- `app/main.go` - Initialize tunnel on startup
- `main.go` - Tunnel lifecycle management

**Config additions:**
```go
WGEnabled      bool   `env:"ORLY_WG_ENABLED" default:"false"`
WGServer       string `env:"ORLY_WG_SERVER"`
WGPrivateKey   string `env:"ORLY_WG_PRIVATE_KEY"`
WGServerPubKey string `env:"ORLY_WG_PUBLIC_KEY"`
WGKeepalive    int    `env:"ORLY_WG_KEEPALIVE" default:"25"`
WGMTU          int    `env:"ORLY_WG_MTU" default:"1280"`
WGReconnect    bool   `env:"ORLY_WG_RECONNECT" default:"true"`
```

### Phase 6: Proxy Server
**Files to create:**
- `cmd/proxy-server/main.go` - Entry point
- `cmd/proxy-server/server.go` - WG server management
- `cmd/proxy-server/auth.go` - Nostr-based auth
- `cmd/proxy-server/registry.go` - User/relay registry
- `cmd/proxy-server/bandwidth.go` - Traffic monitoring
- `cmd/proxy-server/config.go` - Server configuration

**Features:**
- WireGuard server (wireguard-go)
- Nostr event-based authentication (NIP-98 style)
- User registration via signed events
- Relay discovery and assignment
- Bandwidth monitoring and quotas
- Multi-tenant isolation

---

## Key Interfaces

### Tunnel Interface
```go
type Tunnel interface {
    Connect(ctx context.Context) error
    Disconnect() error
    Status() TunnelStatus
    Handoff() (*HandoffState, error)
    Resume(state *HandoffState) error
}
```

### Admin API Interface
```go
type AdminAPI interface {
    Status() (*RelayStatus, error)
    Start() error
    Stop() error
    Restart() error
    Logs(ctx context.Context, lines int) (<-chan LogEntry, error)
}
```

### Tray Interface
```go
type TrayApp interface {
    Run() error
    Quit()
    UpdateStatus(status StatusLevel, tooltip string)
    ShowNotification(title, message string)
}
```

---

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/wailsapp/wails/v2 v2.x.x      // Installer GUI
    golang.zx2c4.com/wireguard v0.x.x        // WireGuard client
    github.com/getlantern/systray v1.x.x     // System tray (or fyne.io/systray)
)
```

---

## Build Commands

```bash
# Standard relay build (unchanged)
CGO_ENABLED=0 go build -o orly

# Relay with tray support
CGO_ENABLED=0 go build -tags tray -o orly

# Installer GUI
cd cmd/orly-setup && wails build -platform linux/amd64,darwin/amd64

# Proxy server
CGO_ENABLED=0 go build -o orly-proxy ./cmd/proxy-server

# All platforms
./scripts/build-all.sh
```

---

## Critical Files Reference

| File | Purpose |
|------|---------|
| `app/config/config.go` | Add WG, tray, admin API config |
| `app/server.go` | Register admin API routes |
| `main.go` | Add --tray flag, WG initialization |
| `scripts/deploy.sh` | Pattern for installer service creation |
| `app/web/src/App.svelte` | Pattern for installer UI |

---

## Backward Compatibility

- Main `orly` binary behavior unchanged without flags
- All new features opt-in via environment variables
- WireGuard gracefully degrades if connection fails
- Tray mode only activates with `--tray` flag
- Admin API can be disabled via `ORLY_ADMIN_API_ENABLED=false`

---

## Success Criteria

1. New user can install via GUI wizard in < 5 minutes
2. README guides user from zero to running relay
3. System tray provides one-click relay management
4. WireGuard tunnel auto-connects and reconnects
5. Proxy server enables home relay exposure without port forwarding
6. All existing functionality preserved
