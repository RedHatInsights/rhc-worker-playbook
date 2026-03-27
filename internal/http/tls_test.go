// Generated with Cursor
package http

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
)

func TestNewTLSConfig_empty(t *testing.T) {
	cfg, err := newTLSConfig(nil, nil, nil)
	if err != nil {
		t.Fatalf("newTLSConfig: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion: got %v", cfg.MinVersion)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs is nil")
	}
	if len(cfg.Certificates) != 0 {
		t.Fatalf("expected no client certificates, got %d", len(cfg.Certificates))
	}
}

func TestNewTLSConfig_with_client_cert(t *testing.T) {
	certPEM, keyPEM := mustCertKeyPair(t)
	cfg, err := newTLSConfig(certPEM, keyPEM, nil)
	if err != nil {
		t.Fatalf("newTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates: got %d want 1", len(cfg.Certificates))
	}
}

func TestNewTLSConfig_invalid_key_pair(t *testing.T) {
	_, err := newTLSConfig([]byte("not pem"), []byte("not pem"), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot parse x509 key pair") {
		t.Fatalf("expected x509 key pair error, got %v", err)
	}
}

func TestNewTLSConfig_extra_ca_pem(t *testing.T) {
	ca := mustPEMCA(t)
	cfg, err := newTLSConfig(nil, nil, [][]byte{ca})
	if err != nil {
		t.Fatalf("newTLSConfig: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs is nil")
	}
}

func TestCreateTLSConfig_missing_files(t *testing.T) {
	_, err := CreateTLSConfig(config.Config{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot read cert-file") {
		t.Fatalf("expected cert read error, got %v", err)
	}
}

func TestCreateTLSConfig_missing_ca(t *testing.T) {
	_, err := CreateTLSConfig(config.Config{
		CARoot: []string{"/nonexistent/ca.pem"},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot read ca-file") {
		t.Fatalf("expected ca read error, got %v", err)
	}
}

func TestCreateTLSConfig_from_files(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := mustCertKeyPair(t)
	caPEM := mustPEMCA(t)

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := CreateTLSConfig(config.Config{
		CertFile: certPath,
		KeyFile:  keyPath,
		CARoot:   []string{caPath},
	})
	if err != nil {
		t.Fatalf("CreateTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates: got %d", len(cfg.Certificates))
	}
}

func mustCertKeyPair(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial := big.NewInt(1)
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"test"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	var certBuf bytes.Buffer
	_ = pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	var keyBuf bytes.Buffer
	_ = pem.Encode(&keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certBuf.Bytes(), keyBuf.Bytes()
}

func mustPEMCA(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"test-ca"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	return buf.Bytes()
}
