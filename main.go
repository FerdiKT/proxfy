package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ferdikt/proxfy/internal/ca"
	"github.com/ferdikt/proxfy/internal/proxy"
	"github.com/ferdikt/proxfy/internal/ui"
)

// Build-time variables (set via ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "start":
		cmdStart()
	case "cert":
		cmdCert()
	case "version", "--version", "-v":
		fmt.Printf("proxfy v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdStart() {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	port := fs.Int("port", 8080, "Proxy server port")
	filter := fs.String("filter", "", "Only log requests matching this domain (e.g. api.example.com)")
	showHeaders := fs.Bool("headers", false, "Show request/response headers")
	showBody := fs.Bool("body", false, "Show request/response body")
	fs.Usage = func() {
		fmt.Println("Usage: proxfy start [options]")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[2:])

	certDir := getCertDir()

	server := proxy.NewServer(*port, certDir, *filter, *showHeaders, *showBody)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ Error: %v\n\n", err)
		os.Exit(1)
	}
}

func cmdCert() {
	fs := flag.NewFlagSet("cert", flag.ExitOnError)
	install := fs.Bool("install", false, "Install CA cert to macOS system trust store (requires sudo)")
	showPath := fs.Bool("path", false, "Print CA certificate file path")
	remove := fs.Bool("remove", false, "Remove CA cert from macOS system trust store (requires sudo)")
	fs.Usage = func() {
		fmt.Println("Usage: proxfy cert [options]")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[2:])

	certDir := getCertDir()
	logger := ui.NewLogger("")

	manager, err := ca.NewManager(certDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *showPath {
		fmt.Println(manager.CACertPath())
		return
	}

	if *install {
		if runtime.GOOS != "darwin" {
			fmt.Fprintln(os.Stderr, "Error: --install is only supported on macOS")
			os.Exit(1)
		}
		certPath := manager.CACertPath()
		fmt.Println("  CA sertifikası macOS trust store'a yükleniyor...")
		fmt.Println("  (sudo şifreniz sorulabilir)")
		fmt.Println()

		cmd := exec.Command("sudo", "security", "add-trusted-cert",
			"-d", "-r", "trustRoot",
			"-k", "/Library/Keychains/System.keychain",
			certPath,
		)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			logger.Error("Sertifika yüklenemedi: %v", err)
			os.Exit(1)
		}
		logger.Info("CA sertifikası başarıyla yüklendi!")
		return
	}

	if *remove {
		if runtime.GOOS != "darwin" {
			fmt.Fprintln(os.Stderr, "Error: --remove is only supported on macOS")
			os.Exit(1)
		}
		certPath := manager.CACertPath()
		fmt.Println("  CA sertifikası trust store'dan kaldırılıyor...")

		cmd := exec.Command("sudo", "security", "remove-trusted-cert",
			"-d", certPath,
		)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			logger.Error("Sertifika kaldırılamadı: %v", err)
			os.Exit(1)
		}
		logger.Info("CA sertifikası başarıyla kaldırıldı!")
		return
	}

	// Default: show cert info
	logger.CertInfo(manager.CACertPath(), manager.CACertFingerprint())
}

func getCertDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".proxfy")
}

func printUsage() {
	fmt.Println()
	fmt.Println("  ⚡ Proxfy — CLI HTTPS Proxy for Mobile Debugging")
	fmt.Println()
	fmt.Println("  USAGE")
	fmt.Println("    proxfy <command> [options]")
	fmt.Println()
	fmt.Println("  COMMANDS")
	fmt.Println("    start       Start the proxy server")
	fmt.Println("    cert        Manage CA certificate")
	fmt.Println("    version     Print version info")
	fmt.Println("    help        Show this help")
	fmt.Println()
	fmt.Println("  START OPTIONS")
	fmt.Println("    --port      Proxy port (default: 8080)")
	fmt.Println("    --filter    Only log matching domain")
	fmt.Println()
	fmt.Println("  CERT OPTIONS")
	fmt.Println("    --path      Print CA cert file path")
	fmt.Println("    --install   Install CA cert to macOS trust store")
	fmt.Println("    --remove    Remove CA cert from macOS trust store")
	fmt.Println()
	fmt.Println("  EXAMPLES")
	fmt.Println("    proxfy start                        Start proxy on port 8080")
	fmt.Println("    proxfy start --port 9090            Start on custom port")
	fmt.Println("    proxfy start --filter api.myapp.com Only log matching requests")
	fmt.Println("    proxfy cert --install               Trust CA cert on macOS")
	fmt.Println()
}
