package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

type MTLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	ServerName string
}

func (m MTLSConfig) ServerTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(m.CertFile, m.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed loading server TLS key pair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	if m.CAFile != "" {
		pool, err := loadCA(m.CAFile)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = pool
	}
	return cfg, nil
}

func (m MTLSConfig) ClientTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(m.CertFile, m.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed loading client TLS key pair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ServerName:   m.ServerName,
	}
	if m.CAFile != "" {
		pool, err := loadCA(m.CAFile)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

func loadCA(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed reading CA bundle %s: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("failed parsing CA PEM from %s", path)
	}
	return pool, nil
}

func FromEnv() MTLSConfig {
	return MTLSConfig{
		CertFile:   os.Getenv("MTLS_CERT"),
		KeyFile:    os.Getenv("MTLS_KEY"),
		CAFile:     os.Getenv("MTLS_CA"),
		ServerName: os.Getenv("MTLS_SERVER_NAME"),
	}
}
