package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ferdikt/proxfy/internal/ca"
	"github.com/ferdikt/proxfy/internal/ui"
)

// hop-by-hop headers that should not be forwarded by a proxy.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// Server is the main MITM proxy server.
type Server struct {
	Port      int
	CertPort  int
	CertDir   string
	Filter    string
	logger    *ui.Logger
	caManager *ca.Manager
	transport *http.Transport
}

// NewServer creates a new proxy server.
func NewServer(port int, certDir, filter string) *Server {
	return &Server{
		Port:     port,
		CertPort: port + 1,
		CertDir:  certDir,
		Filter:   filter,
		logger:   ui.NewLogger(filter),
		transport: &http.Transport{
			TLSClientConfig:   &tls.Config{},
			MaxIdleConns:       200,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true, // Keep original encoding for accurate size counting
		},
	}
}

// Start initializes the CA, starts the cert server, and starts the proxy.
func (s *Server) Start() error {
	var err error
	s.caManager, err = ca.NewManager(s.CertDir)
	if err != nil {
		return fmt.Errorf("CA initialization failed: %w", err)
	}

	localIP := getLocalIP()

	// Start cert download server in background
	go s.serveCertDownload(localIP)

	// Print the banner
	s.logger.Banner(localIP, s.Port, s.CertPort)

	// Start the proxy server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.Port),
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return server.ListenAndServe()
}

// ServeHTTP handles incoming proxy requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
	} else {
		s.handleHTTP(w, r)
	}
}

// handleHTTP handles plain HTTP proxy requests.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Remove hop-by-hop headers
	removeHopByHop(r.Header)

	// Must clear RequestURI for Transport.RoundTrip
	r.RequestURI = ""

	start := time.Now()

	resp, err := s.transport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Proxy Error: "+err.Error(), http.StatusBadGateway)
		if s.shouldLog(r.Host) {
			s.logger.Error("%s %s → %v", r.Method, r.URL, err)
		}
		return
	}
	defer resp.Body.Close()

	// Remove hop-by-hop headers from response
	removeHopByHop(resp.Header)

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy body and count bytes
	n, _ := io.Copy(w, resp.Body)

	if s.shouldLog(r.Host) {
		s.logger.LogRequest(r.Method, r.URL.String(), resp.StatusCode, n, time.Since(start), "http")
	}
}

// handleConnect handles HTTPS CONNECT requests using MITM.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	hostName, _, err := net.SplitHostPort(host)
	if err != nil {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		s.logger.Error("Hijack failed: %v", err)
		return
	}
	defer clientConn.Close()

	// Tell the client the tunnel is established
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// Generate a certificate for the target host
	tlsCert, err := s.caManager.GetCertForHost(hostName)
	if err != nil {
		s.logger.Error("Cert generation for %s failed: %v", hostName, err)
		return
	}

	// TLS handshake with the client (we act as the target server)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*tlsCert},
		NextProtos:   []string{"http/1.1"}, // Force HTTP/1.1 so we can read requests
	}
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		// Client doesn't trust our CA — expected if cert not installed
		return
	}
	defer tlsClientConn.Close()

	// Read HTTP requests from the decrypted TLS connection
	reader := bufio.NewReader(tlsClientConn)

	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			return // connection closed or read error
		}

		// Reconstruct full URL
		req.URL.Scheme = "https"
		req.URL.Host = hostName
		req.RequestURI = "" // Must be empty for RoundTrip

		// Remove hop-by-hop headers
		removeHopByHop(req.Header)

		shouldLog := s.shouldLog(hostName)
		start := time.Now()

		// Forward to actual target server
		resp, err := s.transport.RoundTrip(req)
		if err != nil {
			if shouldLog {
				s.logger.Error("%s %s → %v", req.Method, req.URL, err)
			}
			// Try to send an error response
			errResp := &http.Response{
				StatusCode: http.StatusBadGateway,
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}
			errResp.Write(tlsClientConn)
			return
		}

		// Count body bytes while forwarding
		var bodySize int64
		origBody := resp.Body
		counter := &byteCounter{}
		resp.Body = io.NopCloser(io.TeeReader(origBody, counter))

		// Write the full response back to the client
		writeErr := resp.Write(tlsClientConn)
		bodySize = counter.n
		origBody.Close()

		if shouldLog {
			s.logger.LogRequest(req.Method, req.URL.String(), resp.StatusCode, bodySize, time.Since(start), "https")
		}

		if writeErr != nil {
			return
		}
	}
}

// serveCertDownload starts an HTTP server that serves the CA certificate for download.
// iPhone users can navigate to this URL in Safari to install the cert.
func (s *Server) serveCertDownload(ip string) {
	mux := http.NewServeMux()

	// Serve a simple page with download button
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			certPEM, err := s.caManager.CACertPEM()
			if err != nil {
				http.Error(w, "CA certificate not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/x-x509-ca-cert")
			w.Header().Set("Content-Disposition", `attachment; filename="proxfy-ca.crt"`)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(certPEM)))
			w.Write(certPEM)
			s.logger.Info("📱 CA sertifikası indirildi: %s", r.RemoteAddr)
			return
		}

		// Landing page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, certDownloadPage, ip, s.CertPort)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.CertPort),
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		s.logger.Error("Cert server failed: %v", err)
	}
}

// shouldLog returns true if requests to this host should be logged.
func (s *Server) shouldLog(host string) bool {
	if s.Filter == "" {
		return true
	}
	return strings.Contains(host, s.Filter)
}

// byteCounter counts bytes written to it.
type byteCounter struct {
	n int64
}

func (c *byteCounter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// removeHopByHop removes hop-by-hop headers from the given header map.
func removeHopByHop(h http.Header) {
	for _, header := range hopByHopHeaders {
		h.Del(header)
	}
}

// getLocalIP returns the primary local IP address.
func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	// Prefer en0 (Wi-Fi on macOS) first
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// certDownloadPage is the HTML template for the cert download page.
const certDownloadPage = `<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proxfy — Sertifika Yükle</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'SF Pro', sans-serif;
            background: #0a0a0a;
            color: #e5e5e5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .card {
            background: #1a1a1a;
            border: 1px solid #333;
            border-radius: 20px;
            padding: 40px;
            max-width: 420px;
            width: 100%%;
            text-align: center;
        }
        .logo { font-size: 48px; margin-bottom: 16px; }
        h1 {
            font-size: 24px;
            font-weight: 700;
            color: #00d4aa;
            margin-bottom: 8px;
        }
        .subtitle {
            color: #888;
            font-size: 14px;
            margin-bottom: 32px;
        }
        .download-btn {
            display: block;
            background: linear-gradient(135deg, #00d4aa, #00a885);
            color: #000;
            text-decoration: none;
            padding: 16px 32px;
            border-radius: 14px;
            font-size: 17px;
            font-weight: 600;
            margin-bottom: 32px;
            transition: transform 0.15s ease;
        }
        .download-btn:active { transform: scale(0.97); }
        .steps {
            text-align: left;
            font-size: 14px;
            line-height: 1.8;
            color: #aaa;
        }
        .steps .num {
            display: inline-block;
            width: 24px;
            height: 24px;
            background: #333;
            color: #00d4aa;
            border-radius: 50%%;
            text-align: center;
            line-height: 24px;
            font-size: 12px;
            font-weight: 700;
            margin-right: 8px;
        }
        .steps p { margin-bottom: 12px; }
        .warn {
            margin-top: 24px;
            padding: 12px;
            background: #1e1e00;
            border: 1px solid #444400;
            border-radius: 10px;
            font-size: 12px;
            color: #cccc00;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="logo">⚡</div>
        <h1>Proxfy</h1>
        <p class="subtitle">HTTPS trafiğini okuyabilmek için<br>CA sertifikasını yükleyin</p>

        <a href="/download" class="download-btn">📥 Sertifikayı İndir</a>

        <div class="steps">
            <p><span class="num">1</span>Yukarıdaki butona tıklayın</p>
            <p><span class="num">2</span>Ayarlar → Genel → VPN ve Cihaz Yönetimi</p>
            <p><span class="num">3</span>Proxfy CA profilini yükleyin</p>
            <p><span class="num">4</span>Ayarlar → Genel → Hakkında → Sertifika Güven Ayarları</p>
            <p><span class="num">5</span>Proxfy CA'yı etkinleştirin</p>
        </div>

        <div class="warn">
            ⚠️ Bu sertifika yalnızca geliştirme amaçlıdır. İşiniz bittiğinde sertifikayı kaldırmanız önerilir.
        </div>
    </div>
</body>
</html>`
