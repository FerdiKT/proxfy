package ui

import (
	"fmt"
	"strings"
	"time"
)

// ANSI escape codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	italic = "\033[3m"

	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
	gray    = "\033[90m"

	bgGreen  = "\033[42m"
	bgYellow = "\033[43m"
	bgRed    = "\033[41m"
	bgCyan   = "\033[46m"
	bgBlue   = "\033[44m"
)

// Logger handles all terminal output with colors and formatting.
type Logger struct {
	filter string
}

// NewLogger creates a new Logger instance.
func NewLogger(filter string) *Logger {
	return &Logger{filter: filter}
}

// Banner prints the startup banner with connection info and iPhone setup instructions.
func (l *Logger) Banner(ip string, proxyPort, certPort int) {
	fmt.Println()
	fmt.Printf("  %s%s⚡ Proxfy%s %sv0.1.0%s\n", bold, cyan, reset, dim, reset)
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", dim, reset)
	fmt.Println()
	fmt.Printf("  %sProxy%s        %s%s:%d%s\n", gray, reset, bold+white, ip, proxyPort, reset)
	fmt.Printf("  %sCert Server%s  %shttp://%s:%d%s\n", gray, reset, cyan, ip, certPort, reset)

	if l.filter != "" {
		fmt.Printf("  %sFilter%s       %s%s%s\n", gray, reset, yellow, l.filter, reset)
	}

	fmt.Println()
	fmt.Printf("  %s📱 iPhone Kurulumu:%s\n", bold, reset)
	fmt.Println()
	fmt.Printf("  %s1%s  Wi-Fi → HTTP Proxy → Manual\n", yellow+bold, reset)
	fmt.Printf("     Server: %s%s%s  Port: %s%d%s\n", green, ip, reset, green, proxyPort, reset)
	fmt.Println()
	fmt.Printf("  %s2%s  Safari'de aç → sertifikayı indir:\n", yellow+bold, reset)
	fmt.Printf("     %s%shttp://%s:%d%s\n", bold, cyan, ip, certPort, reset)
	fmt.Println()
	fmt.Printf("  %s3%s  Ayarlar → Genel → VPN ve Cihaz Yönetimi → Sertifikayı Yükle\n", yellow+bold, reset)
	fmt.Println()
	fmt.Printf("  %s4%s  Ayarlar → Genel → Hakkında → Sertifika Güven Ayarları → Etkinleştir\n", yellow+bold, reset)
	fmt.Println()
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", dim, reset)
	fmt.Printf("  %sDurdurmak için Ctrl+C%s\n", dim, reset)
	fmt.Println()
}

// LogRequest logs an HTTP request/response with colors.
func (l *Logger) LogRequest(method, rawURL string, status int, size int64, duration time.Duration, scheme string) {
	now := time.Now().Format("15:04:05")

	methodColor := l.methodColor(method)
	statusColor := l.statusColor(status)

	// Truncate URL
	displayURL := rawURL
	if len(displayURL) > 90 {
		displayURL = displayURL[:87] + "..."
	}

	// Scheme badge
	schemeBadge := fmt.Sprintf("%s%s%s", dim, "http", reset)
	if scheme == "https" {
		schemeBadge = fmt.Sprintf("%s%s🔒%s", green, bold, reset)
	}

	fmt.Printf("  %s%s%s %s%-7s%s %s %-90s %s%3d%s  %s%s%s  %s%s%s\n",
		dim, now, reset,
		methodColor+bold, method, reset,
		schemeBadge, displayURL,
		statusColor, status, reset,
		dim, formatSize(size), reset,
		dim, formatDuration(duration), reset,
	)
}

// Error prints an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s%s✗ %s%s\n", red, bold, msg, reset)
}

// Info prints an info message.
func (l *Logger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s%s● %s%s\n", green, bold, msg, reset)
}

// Warn prints a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s%s⚠ %s%s\n", yellow, bold, msg, reset)
}

// CertInfo prints CA certificate information.
func (l *Logger) CertInfo(certPath, fingerprint string) {
	fmt.Println()
	fmt.Printf("  %s%s🔐 Proxfy CA Certificate%s\n", bold, cyan, reset)
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", dim, reset)
	fmt.Println()
	fmt.Printf("  %sPath%s         %s%s%s\n", gray, reset, white, certPath, reset)
	fmt.Printf("  %sFingerprint%s  %s%s%s\n", gray, reset, dim, truncateFingerprint(fingerprint), reset)
	fmt.Println()
	fmt.Printf("  %smacOS'e yüklemek için:%s\n", bold, reset)
	fmt.Printf("  %s$ sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s%s\n", dim, certPath, reset)
	fmt.Println()
}

func (l *Logger) methodColor(method string) string {
	switch method {
	case "GET":
		return green
	case "POST":
		return yellow
	case "PUT", "PATCH":
		return blue
	case "DELETE":
		return red
	case "OPTIONS":
		return cyan
	case "HEAD":
		return magenta
	default:
		return white
	}
}

func (l *Logger) statusColor(status int) string {
	switch {
	case status >= 500:
		return red + bold
	case status >= 400:
		return yellow
	case status >= 300:
		return cyan
	case status >= 200:
		return green
	default:
		return white
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%6.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%6.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%6d B ", bytes)
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%5.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%4dms", d.Milliseconds())
	}
}

func truncateFingerprint(fp string) string {
	parts := strings.Split(fp, ":")
	if len(parts) > 8 {
		return strings.Join(parts[:8], ":") + ":..."
	}
	return fp
}
