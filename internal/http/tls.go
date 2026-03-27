// Based on yggdrasil's TLS config
// https://github.com/RedHatInsights/yggdrasil/tree/main/internal/config

package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
)

func newTLSConfig(
	certPEMBlock []byte,
	keyPEMBlock []byte,
	CARootPEMBlocks [][]byte,
) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	if len(certPEMBlock) > 0 && len(keyPEMBlock) > 0 {
		cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("cannot parse x509 key pair: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot copy system certificate pool: %w", err)
	}
	for _, data := range CARootPEMBlocks {
		pool.AppendCertsFromPEM(data)
	}
	config.RootCAs = pool

	return config, nil
}

// CreateTLSConfig creates a tls.Config object from the current configuration.
func CreateTLSConfig(conf config.Config) (*tls.Config, error) {
	var certData, keyData []byte
	var err error
	rootCAs := make([][]byte, 0)

	if conf.CertFile != "" && conf.KeyFile != "" {
		certData, err = os.ReadFile(conf.CertFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read cert-file '%v': %w", conf.CertFile, err)
		}

		keyData, err = os.ReadFile(conf.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read key-file '%v': %w", conf.KeyFile, err)
		}
	}

	for _, file := range conf.CARoot {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("cannot read ca-file '%v': %w", file, err)
		}
		rootCAs = append(rootCAs, data)
	}

	tlsConfig, err := newTLSConfig(certData, keyData, rootCAs)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}
