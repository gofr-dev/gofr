package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

var (
	errCACertFileRead = errors.New("failed to read CA certificate file")
	errClientCertLoad = errors.New("failed to load client certificate")
)

type TLSConfig struct {
	CertFile           string
	KeyFile            string
	CACertFile         string
	InsecureSkipVerify bool
}

func createTLSConfig(tlsConf *TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: tlsConf.InsecureSkipVerify,
	}

	if tlsConf.CACertFile != "" {
		caCert, err := os.ReadFile(tlsConf.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errCACertFileRead, err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	if tlsConf.CertFile != "" && tlsConf.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConf.CertFile, tlsConf.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errClientCertLoad, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
