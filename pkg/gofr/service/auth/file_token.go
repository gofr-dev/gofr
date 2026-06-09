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

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

const (
	// DefaultTokenFilePath is the standard Kubernetes projected service account token mount path.
	DefaultTokenFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 -- file path, not a credential

	defaultRefreshInterval = 30 * time.Second
)

var (
	errEmptyTokenFile    = errors.New("token file is empty")
	errTokenUnavailable  = errors.New("no token available")
	errAuthHeaderPresent = errors.New("authorization header already set on request")
)

// FileTokenAuthConfig reads a bearer token from a local file and periodically
// re-reads it to support token rotation (e.g. Kubernetes projected service
// account tokens).
//
// The returned value implements service.Options, service.Observable and
// io.Closer. Call Close to stop the background refresh goroutine; it is safe
// to call Close multiple times.
type FileTokenAuthConfig struct {
	tokenFilePath   string
	refreshInterval time.Duration

	logger logging.Logger

	mu    sync.RWMutex
	token string

	done      chan struct{}
	closeOnce sync.Once
}

// Option configures a FileTokenAuthConfig at construction time. Pass options to
// NewFileTokenAuthConfig to override the defaults (Kubernetes projected SA
// token path, 30s refresh interval).
type Option func(*FileTokenAuthConfig)

// WithTokenFilePath overrides the path the bearer token is read from. The
// default is the standard Kubernetes projected service account token mount at
// DefaultTokenFilePath. Empty values are ignored.
func WithTokenFilePath(path string) Option {
	return func(f *FileTokenAuthConfig) {
		if path != "" {
			f.tokenFilePath = path
		}
	}
}

// WithRefreshInterval overrides how often the token file is re-read. Values
// <= 0 are ignored and the default (30s) is used.
func WithRefreshInterval(d time.Duration) Option {
	return func(f *FileTokenAuthConfig) {
		if d > 0 {
			f.refreshInterval = d
		}
	}
}

// NewFileTokenAuthConfig constructs a FileTokenAuthConfig. With no options it
// reads tokens from DefaultTokenFilePath and refreshes every 30s — the common
// case for Kubernetes projected service account tokens. Override with
// WithTokenFilePath / WithRefreshInterval.
//
// The token file is read eagerly: a missing or empty file returns an error so
// misconfiguration is caught at startup rather than at the first upstream call.
// The logger is supplied automatically by NewHTTPService via the
// service.Observable hook; until it arrives, background-refresh failures are
// silent.
func NewFileTokenAuthConfig(opts ...Option) (*FileTokenAuthConfig, error) {
	f := &FileTokenAuthConfig{
		tokenFilePath:   DefaultTokenFilePath,
		refreshInterval: defaultRefreshInterval,
		done:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(f)
	}

	token, err := readToken(f.tokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token from %s: %w", f.tokenFilePath, err)
	}

	f.token = token

	go f.refreshLoop()

	return f, nil
}

// AddOption implements service.Options.
func (f *FileTokenAuthConfig) AddOption(h service.HTTP) service.HTTP {
	return &fileTokenDecorator{source: f, HTTP: h}
}

// SetLogger implements service.Observable. NewHTTPService calls this with the
// HTTP service's logger so background-refresh failures can be surfaced at WARN
// level. If l does not satisfy logging.Logger (the richer interface with
// Warnf), the logger stays unset and refresh failures remain silent rather
// than panicking.
func (f *FileTokenAuthConfig) SetLogger(l service.Logger) {
	if rich, ok := l.(logging.Logger); ok {
		f.mu.Lock()
		f.logger = rich
		f.mu.Unlock()
	}
}

// SetMetrics implements service.Observable. FileTokenAuthConfig does not emit
// metrics today; the no-op satisfies the interface so the framework can inject
// uniformly.
func (*FileTokenAuthConfig) SetMetrics(service.Metrics) {}

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
				// Keep serving the cached token, but surface the failure so a
				// vanished/locked token file does not stay invisible until an
				// upstream 401.
				f.mu.RLock()
				log := f.logger
				f.mu.RUnlock()

				if log != nil {
					log.Warnf("file token auth: failed to refresh token from %s: %v", f.tokenFilePath, err)
				}

				continue
			}

			f.mu.Lock()
			f.token = token
			f.mu.Unlock()
		}
	}
}

// readToken loads the bearer token from a local file. K8s projected SA tokens
// always live on disk at a known mount path, so we read directly via os.ReadFile
// rather than going through a file.FileSystem abstraction — the indirection was
// not earning its keep for this use case.
func readToken(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is operator-supplied config, not user input
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
		return nil, service.AuthErr{Err: errAuthHeaderPresent, Message: "authorization header already set on request"}
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
