# Antigravity Manager

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/feiyemomo/antigravity-manager)](https://github.com/feiyemomo/antigravity-manager/releases)
[![Platform](https://img.shields.io/badge/platform-Windows%20x64-lightgrey)](https://github.com/feiyemomo/antigravity-manager)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](go.mod)

**[English](README_EN.md) | [简体中文](README.md)**

</div>

A high-availability, high-performance multi-account atomic switcher, real-time quota monitor, and automated breakpoint relay manager for **Google Antigravity (AGY)**. 

Built natively with **Go + Gin**, it provides a zero-dependency portable single binary, seamless background execution with no console flashing, native Windows Credential Manager integration, live in-memory hot-sync via CDP, and a modern bilingual web dashboard that seamlessly matches Antigravity's native aesthetic.

---

## Key Features

1. **Zero-Dependency Portable Single Binary**
   - Pure Go static compilation without bloated Node.js runtimes or sea-prep blobs. Native Windows GUI subsystem execution with zero black console window flashing and minimal memory footprint (~20MB).
2. **Native Windows Credential Manager Integration**
   - Deeply integrates with Windows Credential Manager (`advapi32.dll` - `CredReadW` / `CredWriteW` / `CredDeleteW`) under `gemini:antigravity`. OAuth tokens are encrypted at rest with zero plaintext leakage risk.
3. **In-Memory Live Hot-Sync (No IDE Restarts)**
   - Leverages Chrome DevTools Protocol (CDP) WebSocket to inject updated tokens directly into running Antigravity IDE instances and triggers `cloudCodeService.retrieveUserQuotaSummary`. New accounts and quotas take effect instantly without restarting the editor.
4. **Isolated Profiles & Shared Asset Junctions**
   - Fully isolates sensitive auth identities (`installation_id`, `antigravity-oauth-token`, `settings.json`).
   - Shared data (`conversations/`, `skills/`, etc.) are linked via NTFS directory junctions to `~/.agy_auth/shared/`, preserving conversation history and custom skills across all accounts.
5. **⚡ Auto Switch & Continue Relay (Breakpoint Continuation)**
   - Automatically monitors quota exhaustion: switches to the next healthy account -> atomically restarts language service -> navigates back to the active conversation -> simulates DOM interaction and sends a native `Enter` event with "继续" (Continue) automatically.
   - Includes UI toggle switch and threshold safeguard.
6. **Native Visual Dashboard & Bilingual Support**
   - 1:1 reproduction of Antigravity's native design system (Dark & Light themes). Fully driven by CSS variables for 0-latency instant theme color switching.
   - Built-in **🌐 Bilingual (Chinese / English)** one-click language toggle with persistent local storage.

---

## Architecture

```text
antigravity-manager/
├── cmd/
│   └── agy-tools/
│       └── main.go              # Cobra CLI & Windows GUI entry point
├── pkg/
│   ├── config/                  # Path constants & configuration (~/.agy_auth, ~/.gemini)
│   ├── types/                   # Domain models (Account, QuotaData, ProfilePayload, DoctorReport)
│   ├── core/
│   │   ├── credential/          # Windows Credential Manager API wrapper
│   │   ├── process/             # Process detection & language server restart
│   │   ├── symlink/             # NTFS directory junction management
│   │   ├── profile/             # Profile isolation read/write engine
│   │   ├── livesync/            # Chrome DevTools Protocol (CDP) live sync
│   │   ├── switcher/            # Atomic transaction switcher with snapshot rollback
│   │   └── doctor/              # System health diagnostic & self-healing
│   ├── auth/                    # Google OAuth 2.0 PKCE flow & project/tier resolver
│   ├── quota/                   # CloudCode API quota fetching & grouped parser
│   ├── store/                   # Thread-safe persistent account storage & auto-refresh
│   ├── rotator/                 # Auto-rotation & breakpoint relay orchestrator
│   └── server/
│       ├── server.go            # Gin Web engine & middleware setup
│       ├── handlers/            # REST API controllers (accounts, switch, oauth, settings)
│       └── web/
│           ├── embed.go         # Static frontend assets embedding (Go embed.FS)
│           └── index.html       # Bilingual dual-theme reactive web dashboard
```

---

## Download & Getting Started

### 1. Download Prebuilt Release
Download the latest portable archive from [GitHub Releases](https://github.com/feiyemomo/antigravity-manager/releases):
- `Antigravity-Manager-v2.0.0-windows-amd64.zip`

Extracted files:
- **`Antigravity-Manager.exe`**: **Double-click to run**. Runs silently in background without console windows and automatically opens the dashboard in your default browser.
- **`agy-tools.exe`**: Command-line CLI version for terminal power users and automation scripts.
- **`start-manager.bat` / `stop-manager.bat`**: Convenient scripts to start and safely stop background services.

### 2. Build from Source

Requirements: Go 1.22 or higher.

```bash
# Clone the repository
git clone https://github.com/feiyemomo/antigravity-manager.git
cd antigravity-manager

# Build GUI single binary (Recommended for daily use, no console window)
go build -ldflags="-H windowsgui -s -w" -o Antigravity-Manager.exe ./cmd/agy-tools

# Build CLI binary
go build -ldflags="-s -w" -o agy-tools.exe ./cmd/agy-tools
```

---

## CLI Command Reference

### Starting the Server
```bash
# Start background service and web dashboard (default port: 38080)
agy-tools start

# Custom port and prevent auto-opening browser
agy-tools start --port 38080 --no-open
```

### Account Management
```bash
# Login and authorize a new Google account via OAuth 2.0
agy-tools login

# List all authenticated accounts and their remaining quotas
agy-tools accounts list
```

### Account Switching & Diagnostics
```bash
# Switch to a target account with live hot-sync into running IDE
agy-tools switch user@gmail.com --live

# Rotate to the next available healthy account
agy-tools rotate

# Run full health diagnostics and auto-repair symlinks
agy-tools doctor --fix
```

---

## License

This project is licensed under the [MIT License](LICENSE).  
Copyright (c) 2026 feiyemomo
