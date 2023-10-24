package gofr

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gofr.dev/pkg/log"
)

// HTTPS as the name suggest, this type is used for starting a HTTPS server
type HTTPS struct {
	Port            int
	TLSConfig       *tls.Config
	CertificateFile string
	KeyFile         string
}

const (
	ReadTimeOut  = 15
	WriteTimeOut = 60
	IdleTimeOut  = 15
)

// StartServer starts an HTTPS server, using the parameters from the receiver
func (h *HTTPS) StartServer(logger log.Logger, router http.Handler) {
	if h.TLSConfig == nil {
		h.TLSConfig = h.perfectSSLScoreConfig()
	}

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(h.Port),
		ReadTimeout:  ReadTimeOut * time.Second,
		WriteTimeout: WriteTimeOut * time.Second,
		IdleTimeout:  IdleTimeOut * time.Second,
		TLSConfig:    h.TLSConfig,
		Handler:      router,
	}

	certFile, _ := filepath.Abs(h.CertificateFile)

	_, err := os.Stat(certFile)
	if err != nil {
		logger.Error("error in certificate file  ", err)
		return
	}

	keyFile, _ := filepath.Abs(h.KeyFile)

	_, err = os.Stat(keyFile)
	if err != nil {
		logger.Error("error in certificate key  ", err)
		return
	}

	logger.Logf("starting https server at :%v", h.Port)

	err = srv.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		logger.Error("unable to start HTTPS Server", err)
	}
}

//nolint:gosec // We are using insecure tls as it is required by http2.
func (h *HTTPS) perfectSSLScoreConfig() *tls.Config {
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
}
