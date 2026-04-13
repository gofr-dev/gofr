// Package auth provides authentication options for outgoing HTTP service calls
// that live outside the core service package. New authentication types should
// be added here as service.Options implementations.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/service"
)

const (
	// DefaultTokenFilePath is the standard Kubernetes projected service account token mount path.
	DefaultTokenFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 -- file path, not a credential

	defaultRefreshInterval = 30 * time.Second
)

var (
	errEmptyTokenFile   = errors.New("token file is empty")
	errTokenUnavailable = errors.New("no token available")
)

// FileTokenAuthConfig reads a bearer token from a file and periodically re-reads it
// to support token rotation (e.g. Kubernetes projected service account tokens).
//
// The returned value implements service.Options and io.Closer. Call Close to stop
// the background refresh goroutine; it is safe to call Close multiple times.
type FileTokenAuthConfig struct {
	tokenFilePath   string
	refreshInterval time.Duration

	mu    sync.RWMutex
	token string

	done      chan struct{}
	closeOnce sync.Once
}

// NewFileTokenAuthConfig constructs a FileTokenAuthConfig. If tokenFilePath is empty
// it defaults to DefaultTokenFilePath. If refreshInterval is <= 0 it defaults to 30s.
func NewFileTokenAuthConfig(tokenFilePath string, refreshInterval time.Duration) (*FileTokenAuthConfig, error) {
	if tokenFilePath == "" {
		tokenFilePath = DefaultTokenFilePath
	}

	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}

	token, err := readToken(tokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token from %s: %w", tokenFilePath, err)
	}

	f := &FileTokenAuthConfig{
		tokenFilePath:   tokenFilePath,
		refreshInterval: refreshInterval,
		token:           token,
		done:            make(chan struct{}),
	}

	go f.refreshLoop()

	return f, nil
}

// AddOption implements service.Options.
func (f *FileTokenAuthConfig) AddOption(h service.HTTP) service.HTTP {
	return &fileTokenDecorator{source: f, HTTP: h}
}

// Close stops the background refresh goroutine. It is safe to call multiple times.
func (f *FileTokenAuthConfig) Close() error {
	f.closeOnce.Do(func() {
		close(f.done)
	})

	return nil
}

func (f *FileTokenAuthConfig) currentToken() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.token == "" {
		return "", errTokenUnavailable
	}

	return f.token, nil
}

func (f *FileTokenAuthConfig) refreshLoop() {
	ticker := time.NewTicker(f.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.done:
			return
		case <-ticker.C:
			token, err := readToken(f.tokenFilePath)
			if err != nil {
				continue
			}

			f.mu.Lock()
			f.token = token
			f.mu.Unlock()
		}
	}
}

func readToken(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is configured by the application
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errEmptyTokenFile
	}

	return token, nil
}

// fileTokenDecorator wraps a service.HTTP and injects a bearer token read from a file.
// It exposes Unwrap so that ConnectionPoolConfig / CircuitBreakerConfig / RetryConfig
// can reach the underlying *httpService through the service package's extractHTTPService.
type fileTokenDecorator struct {
	source *FileTokenAuthConfig
	service.HTTP
}

func (d *fileTokenDecorator) Unwrap() service.HTTP {
	return d.HTTP
}

func (d *fileTokenDecorator) inject(headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if existing, ok := headers[service.AuthHeader]; ok && existing != "" {
		return nil, fmt.Errorf("value %v already exists for header %v", existing, service.AuthHeader)
	}

	token, err := d.source.currentToken()
	if err != nil {
		return nil, err
	}

	headers[service.AuthHeader] = "Bearer " + token

	return headers, nil
}

func (d *fileTokenDecorator) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return d.GetWithHeaders(ctx, path, queryParams, nil)
}

func (d *fileTokenDecorator) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	headers, err := d.inject(headers)
	if err != nil {
		return nil, err
	}

	return d.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (d *fileTokenDecorator) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return d.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (d *fileTokenDecorator) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := d.inject(headers)
	if err != nil {
		return nil, err
	}

	return d.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (d *fileTokenDecorator) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return d.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (d *fileTokenDecorator) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := d.inject(headers)
	if err != nil {
		return nil, err
	}

	return d.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (d *fileTokenDecorator) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return d.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (d *fileTokenDecorator) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := d.inject(headers)
	if err != nil {
		return nil, err
	}

	return d.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (d *fileTokenDecorator) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return d.DeleteWithHeaders(ctx, path, body, nil)
}

func (d *fileTokenDecorator) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers, err := d.inject(headers)
	if err != nil {
		return nil, err
	}

	return d.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}
