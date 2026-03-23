# ⚡ Proxfy

CLI-based HTTPS proxy for mobile debugging. A lightweight alternative to Charles Proxy and Proxyman — no GUI needed.

## Features

- 🔒 **HTTPS MITM Interception** — Decrypt and inspect HTTPS traffic
- 📱 **iPhone/iPad Ready** — Built-in CA cert server for easy mobile setup
- 🎨 **Color-coded Logs** — Method, status, size, and duration at a glance
- 🔍 **Domain Filtering** — Focus on specific APIs with `--filter`
- 🚀 **Zero Dependencies** — Single binary, pure Go standard library
- ⚡ **Fast** — ECDSA P-256 certs, goroutine-per-connection

## Install

```bash
# Homebrew (macOS & Linux)
brew install ferdikt/tap/proxfy

# Or with Go
go install github.com/ferdikt/proxfy@latest

# Or build from source
git clone https://github.com/ferdikt/proxfy.git
cd proxfy
make install
```

## Quick Start

```bash
# 1. Start the proxy
proxfy start

# 2. Install CA cert on macOS (optional, for local testing)
proxfy cert --install

# 3. Configure your iPhone (see below)
```

## iPhone Setup

1. **Start Proxfy** on your Mac:
   ```bash
   proxfy start
   ```

2. **Configure iPhone Wi-Fi proxy:**
   - Settings → Wi-Fi → (your network) → Configure Proxy → Manual
   - Server: `<your Mac's IP>` (shown in Proxfy output)
   - Port: `8080`

3. **Install CA certificate:**
   - Open Safari on iPhone
   - Navigate to `http://<your Mac's IP>:8081`
   - Tap "Download Certificate"

4. **Trust the certificate:**
   - Settings → General → VPN & Device Management → Install Proxfy CA
   - Settings → General → About → Certificate Trust Settings → Enable Proxfy CA

## Usage

```
proxfy <command> [options]

COMMANDS
  start       Start the proxy server
  cert        Manage CA certificate
  version     Print version info
  help        Show this help

START OPTIONS
  --port      Proxy port (default: 8080)
  --filter    Only log requests matching domain

CERT OPTIONS
  --path      Print CA cert file path
  --install   Install CA cert to macOS trust store
  --remove    Remove CA cert from macOS trust store
```

## Examples

```bash
# Start on default port
proxfy start

# Start on custom port
proxfy start --port 9090

# Only log API requests
proxfy start --filter api.myapp.com

# Show CA cert info
proxfy cert

# Install CA to macOS keychain
proxfy cert --install
```

## How It Works

```
iPhone                    Proxfy                     Server
  │                         │                          │
  │──── CONNECT host:443 ──→│                          │
  │←── 200 Established ─────│                          │
  │                         │                          │
  │◄═══ TLS (fake cert) ═══►│◄═══ TLS (real cert) ═══►│
  │                         │                          │
  │──── GET /api/data ──────→│──── GET /api/data ──────→│
  │←── 200 {json...} ───────│←── 200 {json...} ───────│
  │                         │                          │
  │       (logged & displayed in terminal)             │
```

1. iPhone sends a `CONNECT` request to Proxfy
2. Proxfy generates a TLS certificate for the target domain (signed by its CA)
3. Proxfy does a TLS handshake with iPhone using the fake cert
4. Proxfy connects to the real server with a real TLS connection
5. All traffic flows through Proxfy and is logged in the terminal

## Cleanup

When done debugging:

```bash
# Remove CA from macOS trust store
proxfy cert --remove

# Remove CA certificate files
rm -rf ~/.proxfy

# On iPhone: Settings → General → VPN & Device Management → Remove Proxfy CA
```

## License

MIT
