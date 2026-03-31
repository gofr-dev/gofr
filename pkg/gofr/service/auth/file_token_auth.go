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
	DefaultTokenFilePath   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultRefreshInterval = 30 * time.Second
)

// FileTokenProvider extends AuthProvider with lifecycle management for the background
// token refresh goroutine.
type FileTokenProvider interface {
	AuthProvider
	service.Options
	Close() error
}

type fileTokenAuthConfig struct {
	fs              file.FileSystem
	tokenFilePath   string
	refreshInterval time.Duration
	mu              sync.RWMutex
	token           string
	done            chan struct{}
	closeOnce       sync.Once
}

// NewFileTokenAuthConfig creates an auth provider that reads a bearer token from a file
// and periodically re-reads it to support token rotation (e.g., Kubernetes projected
// service account tokens).
//
// If tokenFilePath is empty, it defaults to DefaultTokenFilePath.
// If refreshInterval is zero or negative, it defaults to 30s.
func NewFileTokenAuthConfig(fs file.FileSystem, tokenFilePath string,
	refreshInterval time.Duration) (FileTokenProvider, error) {
	if fs == nil {
		return nil, AuthErr{Message: "file system is required"}
	}

	if tokenFilePath == "" {
		tokenFilePath = DefaultTokenFilePath
	}

	token, err := readToken(fs, tokenFilePath)
	if err != nil {
		return nil, AuthErr{Err: err, Message: fmt.Sprintf("failed to read token from %s", tokenFilePath)}
	}

	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}

	f := &fileTokenAuthConfig{
		fs:              fs,
		tokenFilePath:   tokenFilePath,
		refreshInterval: refreshInterval,
		token:           token,
		done:            make(chan struct{}),
	}

	go f.refreshLoop()

	return f, nil
}

func (*fileTokenAuthConfig) GetHeaderKey() string {
	return service.AuthHeader
}

func (f *fileTokenAuthConfig) GetHeaderValue(_ context.Context) (string, error) {
	f.mu.RLock()
	token := f.token
	f.mu.RUnlock()

	if token == "" {
		return "", AuthErr{Message: "no token available"}
	}

	return "Bearer " + token, nil
}

func (f *fileTokenAuthConfig) AddOption(h service.HTTP) service.HTTP {
	adapter := &authOptionAdapter{provider: f}

	return &authProvider{
		auth: adapter.addHeader,
		HTTP: h,
	}
}

func (f *fileTokenAuthConfig) Close() error {
	f.closeOnce.Do(func() {
		close(f.done)
	})

	return nil
}

func (f *fileTokenAuthConfig) refreshLoop() {
	ticker := time.NewTicker(f.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.done:
			return
		case <-ticker.C:
			token, err := readToken(f.fs, f.tokenFilePath)
			if err != nil {
				// Keep last good token on transient read errors.
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
		return "", fmt.Errorf("token file is empty")
	}

	return token, nil
}
