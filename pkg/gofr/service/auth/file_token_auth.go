package auth

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/service"
)

const (
	// DefaultTokenFilePath is the standard Kubernetes projected service account token mount path.
	DefaultTokenFilePath   = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 -- file path, not a credential
	defaultRefreshInterval = 30 * time.Second
)

type fileTokenSource struct {
	fs              file.FileSystem
	tokenFilePath   string
	refreshInterval time.Duration
	mu              sync.RWMutex
	token           string
	done            chan struct{}
	closeOnce       sync.Once
	logger          service.Logger
	metrics         service.Metrics
}

// NewFileTokenAuthConfig creates a service.Options that reads a bearer token from a file
// and periodically re-reads it to support token rotation (e.g., Kubernetes projected
// service account tokens).
//
// The returned value also implements io.Closer — call Close() to stop the background refresh.
//
// If tokenFilePath is empty, it defaults to DefaultTokenFilePath.
// If refreshInterval is zero or negative, it defaults to 30s.
func NewFileTokenAuthConfig(fs file.FileSystem, tokenFilePath string,
	refreshInterval time.Duration) (service.Options, error) {
	if fs == nil {
		return nil, Err{Message: "file system is required"}
	}

	if tokenFilePath == "" {
		tokenFilePath = DefaultTokenFilePath
	}

	token, err := readToken(fs, tokenFilePath)
	if err != nil {
		return nil, Err{Err: err, Message: fmt.Sprintf("failed to read token from %s", tokenFilePath)}
	}

	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}

	f := &fileTokenSource{
		fs:              fs,
		tokenFilePath:   tokenFilePath,
		refreshInterval: refreshInterval,
		token:           token,
		done:            make(chan struct{}),
	}

	go f.refreshLoop()

	return f, nil
}

func (f *fileTokenSource) Token(_ context.Context) (string, error) {
	f.mu.RLock()
	token := f.token
	f.mu.RUnlock()

	if token == "" {
		return "", Err{Message: "no token available"}
	}

	return token, nil
}

func (f *fileTokenSource) AddOption(h service.HTTP) service.HTTP {
	return NewBearerAuthOption(f).AddOption(h)
}

func (f *fileTokenSource) UseLogger(logger service.Logger) {
	f.logger = logger
}

func (f *fileTokenSource) UseMetrics(metrics service.Metrics) {
	f.metrics = metrics
}

func (f *fileTokenSource) Close() error {
	f.closeOnce.Do(func() {
		close(f.done)
	})

	return nil
}

func (f *fileTokenSource) refreshLoop() {
	ticker := time.NewTicker(f.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.done:
			return
		case <-ticker.C:
			token, err := readToken(f.fs, f.tokenFilePath)
			if err != nil {
				if f.logger != nil {
					f.logger.Log(fmt.Sprintf("failed to refresh token from %s: %v", f.tokenFilePath, err))
				}

				continue
			}

			f.mu.Lock()
			f.token = token
			f.mu.Unlock()
		}
	}
}

func readToken(fs file.FileSystem, path string) (string, error) {
	f, err := fs.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errEmptyTokenFile
	}

	return token, nil
}
