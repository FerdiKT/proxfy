# Proxfy — Project Context

## Overview
Proxfy is a CLI-based HTTPS MITM proxy for mobile debugging, written in pure Go with zero external dependencies. It's a lightweight alternative to Charles Proxy and Proxyman.

## Architecture

```
main.go                     CLI entry point, subcommand routing (start, cert, version, help)
internal/
  ca/ca.go                  CA certificate management + per-host TLS cert generation
  proxy/proxy.go            HTTP/HTTPS MITM proxy server + cert download HTTP server
  ui/ui.go                  Colored terminal output using ANSI escape codes
```

### Key Components

**CA Manager** (`internal/ca/ca.go`):
- Generates ECDSA P-256 CA certificate on first run, stored in `~/.proxfy/`
- Signs per-host TLS certificates on-the-fly for MITM interception
- Thread-safe in-memory cert cache using `sync.Map`
- Key methods: `NewManager()`, `GetCertForHost()`, `CACertPEM()`

**Proxy Server** (`internal/proxy/proxy.go`):
- Uses `http.Server` with a custom `ServeHTTP` handler
- HTTP requests: forwarded via `http.Transport.RoundTrip()`
- HTTPS requests: intercepts `CONNECT`, hijacks connection, performs TLS MITM
- Forces HTTP/1.1 via `NextProtos` in TLS config (no HTTP/2 in MITM tunnel)
- Cert download server runs on `port+1` with HTML landing page for mobile
- Hop-by-hop headers are stripped from forwarded requests/responses

**UI Logger** (`internal/ui/ui.go`):
- Pure ANSI escape codes, no external color libraries
- Color-coded HTTP methods (GET=green, POST=yellow, DELETE=red, etc.)
- Color-coded status codes (2xx=green, 3xx=cyan, 4xx=yellow, 5xx=red)
- Startup banner includes iPhone setup instructions in Turkish

## Tech Stack
- **Language**: Go 1.21+
- **Dependencies**: None (pure standard library)
- **Crypto**: ECDSA P-256 for CA and per-host certs
- **CLI**: `flag` package with manual subcommand routing (no cobra)

## MITM Flow
1. Client sends `CONNECT host:443`
2. Proxy hijacks TCP connection, replies `200 Connection Established`
3. Proxy generates TLS cert for target host (signed by Proxfy CA)
4. TLS handshake with client using fake cert
5. Reads HTTP/1.1 requests from decrypted stream
6. Forwards each request to real server via `http.Transport`
7. Streams response back, counts bytes, logs to terminal

## Conventions
- **No external dependencies** — everything uses Go standard library
- **Turkish UI** — user-facing strings (banner, cert page) are in Turkish
- **Config dir**: `~/.proxfy/` stores CA cert and key
- **Port convention**: proxy on `--port` (default 8080), cert server on `port+1`
- **Error handling**: wrap with `fmt.Errorf("context: %w", err)`
- **Logging**: all output goes through `ui.Logger` methods

## Build & Run
```bash
go build -o proxfy .          # Build
./proxfy start                # Start proxy on port 8080
./proxfy start --port 9090    # Custom port
./proxfy start --filter x.com # Filter by domain
./proxfy cert                 # Show CA cert info
./proxfy cert --install       # Install CA to macOS keychain
```

## File Locations
- CA Certificate: `~/.proxfy/proxfy-ca.pem`
- CA Private Key: `~/.proxfy/proxfy-ca-key.pem` (0600 permissions)
- Binary: `./proxfy` (after build)

## Adding New Features
When extending Proxfy, follow these patterns:
1. **New subcommand**: Add case in `main.go` switch + `cmdXxx()` function
2. **New proxy feature**: Extend `Server` struct and `ServeHTTP` in `proxy.go`
3. **New output format**: Add method to `ui.Logger` in `ui.go`
4. **New cert operation**: Add method to `ca.Manager` in `ca.go`
