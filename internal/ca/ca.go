package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager handles CA certificate generation and per-host cert signing.
type Manager struct {
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
	dir   string
	cache sync.Map // host -> *tls.Certificate
}

// NewManager loads an existing CA or creates a new one in the given directory.
func NewManager(dir string) (*Manager, error) {
	m := &Manager{dir: dir}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	certPath := filepath.Join(dir, "proxfy-ca.pem")
	keyPath := filepath.Join(dir, "proxfy-ca-key.pem")

	if _, err := os.Stat(certPath); err == nil {
		if err := m.load(certPath, keyPath); err != nil {
			return nil, fmt.Errorf("load CA: %w", err)
		}
		return m, nil
	}

	if err := m.generate(certPath, keyPath); err != nil {
		return nil, fmt.Errorf("generate CA: %w", err)
	}
	return m, nil
}

func (m *Manager) load(certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	m.ca, err = x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}
	m.caKey = key

	return nil
}

func (m *Manager) generate(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Proxfy CA",
			Organization: []string{"Proxfy"},
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	// Save certificate
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certOut.Close()
		return err
	}
	certOut.Close()

	// Save private key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		keyOut.Close()
		return err
	}
	keyOut.Close()

	m.ca, err = x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}
	m.caKey = key

	return nil
}

// GetCertForHost generates a TLS certificate for a specific host, signed by the CA.
// Results are cached in memory.
func (m *Manager) GetCertForHost(host string) (*tls.Certificate, error) {
	if cert, ok := m.cache.Load(host); ok {
		return cert.(*tls.Certificate), nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Deterministic serial from host
	hash := sha256.Sum256([]byte(host + time.Now().String()))
	serial := new(big.Int).SetBytes(hash[:16])

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{"Proxfy"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Add SAN
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, m.ca, &key.PublicKey, m.caKey)
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	m.cache.Store(host, cert)
	return cert, nil
}

// CACertPath returns the path to the CA certificate file.
func (m *Manager) CACertPath() string {
	return filepath.Join(m.dir, "proxfy-ca.pem")
}

// CACertFingerprint returns the SHA-256 fingerprint of the CA certificate.
func (m *Manager) CACertFingerprint() string {
	hash := sha256.Sum256(m.ca.Raw)
	hex := hex.EncodeToString(hash[:])

	// Format as colon-separated pairs
	result := ""
	for i := 0; i < len(hex); i += 2 {
		if i > 0 {
			result += ":"
		}
		end := i + 2
		if end > len(hex) {
			end = len(hex)
		}
		result += hex[i:end]
	}
	return result
}

// CACertPEM returns the PEM-encoded CA certificate bytes.
func (m *Manager) CACertPEM() ([]byte, error) {
	return os.ReadFile(m.CACertPath())
}
