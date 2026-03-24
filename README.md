<p align="center">
  <img src="assets/hero-banner.svg" alt="Proxfy — CLI HTTPS Proxy" width="100%">
</p>

# proxfy

A powerful CLI HTTPS proxy for mobile debugging · MITM interception · Single binary · Zero dependencies

## ✨ Why Proxfy?

Stop paying for GUI proxy tools. Start inspecting traffic from your terminal.

| | Proxfy | Charles Proxy | Proxyman |
|---|---|---|---|
| **Price** | Free & open-source | $50 | $69/yr |
| **Install** | `brew install` | Download + license | Download + license |
| **GUI Required** | No — pure CLI | Yes | Yes |
| **Binary Size** | ~6 MB | ~80 MB | ~60 MB |
| **Dependencies** | Zero | Java Runtime | None |
| **CI/SSH Friendly** | ✅ | ❌ | ❌ |
| **iPhone Setup** | Built-in cert server | Manual | Manual |

## 📦 Installation

```bash
brew tap FerdiKT/tap
brew install proxfy
```

```bash
go install github.com/ferdikt/proxfy@latest
```

```bash
git clone https://github.com/FerdiKT/proxfy.git
cd proxfy
make install
```

## 🚀 Quickstart

Get up and running in 2 minutes.

### 1️⃣ Start the proxy

```bash
proxfy start
```

### 2️⃣ Connect your iPhone

Open **Settings → Wi-Fi → (your network) → Configure Proxy → Manual** and enter the IP and port shown in the terminal.

### 3️⃣ Install the CA certificate

Open **Safari** on your iPhone and navigate to the cert server URL shown in the terminal (e.g. `http://192.168.1.x:8081`). Then:

- **Settings → General → VPN & Device Management** → Install Proxfy CA
- **Settings → General → About → Certificate Trust Settings** → Enable Proxfy CA

### 4️⃣ Inspect traffic

All HTTP/HTTPS traffic from your iPhone now flows through Proxfy and is logged in your terminal.

```bash
# See full request/response headers
proxfy start --headers

# See headers + body (with JSON pretty-printing)
proxfy start --headers --body

# Focus on a specific API
proxfy start --headers --filter api.myapp.com
```

## 🗺️ Command Reference

### `proxfy start`

Start the MITM proxy server.

```
proxfy start [options]

Options:
  --port      Proxy port (default: 8080)
  --filter    Only log requests matching this domain
  --headers   Show request/response headers
  --body      Show request/response body
```

### `proxfy cert`

Manage the CA certificate.

```
proxfy cert [options]

Options:
  --path      Print CA cert file path
  --install   Install CA cert to macOS trust store (requires sudo)
  --remove    Remove CA cert from macOS trust store (requires sudo)
```

### `proxfy version`

```bash
$ proxfy version
proxfy v0.1.0 (darwin/arm64)
```

## 🔐 MITM Flow

```
iPhone                     Proxfy                      Server
  │                          │                           │
  │── CONNECT host:443 ─────→│                           │
  │←── 200 Established ──────│                           │
  │                          │                           │
  │◄══ TLS (Proxfy cert) ══►│◄══ TLS (real cert) ══════►│
  │                          │                           │
  │── GET /api/data ─────────→│── GET /api/data ─────────→│
  │←── 200 {json} ───────────│←── 200 {json} ────────────│
  │                          │                           │
  │    (decrypted, logged, and displayed in terminal)    │
```

1. iPhone sends `CONNECT` to establish HTTPS tunnel
2. Proxfy generates a TLS certificate for the target host (signed by its CA)
3. Client-side TLS handshake using the Proxfy-signed certificate
4. Server-side TLS handshake using the real certificate
5. All requests are decrypted, forwarded, logged, and re-encrypted

## 🔍 Header Inspection

Sensitive headers are automatically highlighted with `★` so you can spot tokens instantly:

```
10:24:01 POST   🔒 api.myapp.com/v2/auth/login     200  1.2 KB  326ms
     ── Request Headers ──
       Content-Type: application/json
     ★ Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.eyJ1c2VyX2lk...
       User-Agent: MyApp/3.2.1

     ── Response Headers ──
       Content-Type: application/json
     ★ Set-Cookie: session=abc123; Path=/; HttpOnly
```

Highlighted headers: `Authorization` · `Cookie` · `Set-Cookie` · `X-Access-Token` · `X-Auth-Token` · `X-Api-Key` · `X-Csrf-Token`

Token-bearing headers (`Authorization`, `X-Access-Token`, `X-Auth-Token`) are **never truncated** so you can copy the full value.

## ⚙️ Configuration

| Item | Location |
|------|----------|
| CA Certificate | `~/.proxfy/proxfy-ca.pem` |
| CA Private Key | `~/.proxfy/proxfy-ca-key.pem` |
| Proxy Port | `--port` flag (default: `8080`) |
| Cert Server | Automatically on `port + 1` |

## 🧹 Cleanup

When done debugging, remove the CA certificate:

```bash
# Remove from macOS trust store
proxfy cert --remove

# Delete certificate files
rm -rf ~/.proxfy

# On iPhone:
# Settings → General → VPN & Device Management → Remove Proxfy CA
```

## 🏗️ Architecture

```
proxfy
├── main.go                 CLI entry point, subcommand routing
└── internal/
    ├── ca/ca.go             CA cert management + per-host cert generation
    ├── proxy/proxy.go       HTTP/HTTPS MITM proxy + cert download server
    └── ui/ui.go             Color-coded terminal output (ANSI)
```

**Zero external dependencies** — built entirely on Go standard library.

## 🤝 Contributing

1. Fork the repo and create a feature branch
2. Make your changes
3. Run checks: `go vet ./...`
4. Submit a PR

## 📄 License

MIT

---

<p align="center">
  <sub>Built with ❤️ for mobile developers who prefer the terminal</sub>
</p>
