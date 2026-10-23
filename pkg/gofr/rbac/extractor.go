package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

// Extractor returns the permissions the caller of r holds, from a source the application owns, such
// as a database table or a policy service. It runs with a deadline on r.Context(), and should pass
// that context to every call it makes.
//
// An error denies the request. An error with a StatusCode() int method sets the response status,
// ErrRoleNotFound (or an error wrapping it) is a 401, and any other error is a 403.
//
// A panic in the extractor is recovered by the logging middleware that wraps the app's handlers; a
// caller using Middleware directly must recover it itself.
type Extractor func(c *container.Container, r *http.Request) ([]string, error)

// Option changes how LoadPermissions builds a Config.
type Option func(*Config)

// defaultExtractorTimeout bounds an extractor call when no timeout is given.
const defaultExtractorTimeout = 5 * time.Second

// Values of the reason label on the extraction failures metric, in extractor mode.
const (
	reasonExtractorError   = "extractor_error"
	reasonExtractorTimeout = "extractor_timeout"
)

var (
	// errNilExtractor is returned when WithExtractor is given a nil function.
	errNilExtractor = errors.New("RBAC extractor is nil")

	// errExtractorWithClaimPath is returned when a config names a JWT claim to read and an extractor
	// is set too: the two would disagree about where permissions come from.
	errExtractorWithClaimPath = errors.New("jwtClaimPath and permissionsClaimPath cannot be combined with an extractor")
)

// WithExtractor makes the config read permissions from fn instead of a header or a JWT claim. fn
// receives c, and runs under timeout; a timeout that is zero or negative means 5 seconds.
func WithExtractor(fn Extractor, c *container.Container, timeout time.Duration) Option {
	if timeout <= 0 {
		timeout = defaultExtractorTimeout
	}

	return func(cfg *Config) {
		cfg.extractorSet = true
		cfg.extractor = fn
		cfg.container = c
		cfg.extractorTimeout = timeout
	}
}

// validateExtractor rejects an extractor that is nil, or set beside a claim path.
func (c *Config) validateExtractor() error {
	if !c.extractorSet {
		return nil
	}

	if c.extractor == nil {
		return errNilExtractor
	}

	if c.JWTClaimPath != "" || c.PermissionsClaimPath != "" {
		return errExtractorWithClaimPath
	}

	return nil
}

// extractHeldFromExtractor calls the application's extractor under its deadline. The request is
// denied whenever the deadline passes or the request is canceled, even if the extractor returned
// permissions: an extractor that ignored its context is never allowed to grant late.
func extractHeldFromExtractor(r *http.Request, config *Config) (held, error) {
	ctx, cancel := context.WithTimeout(r.Context(), config.extractorTimeout)
	defer cancel()

	perms, err := config.extractor(config.container, r.WithContext(ctx))

	if ctxErr := ctx.Err(); ctxErr != nil {
		return held{}, &extractionFailure{
			sentinel: fmt.Errorf("%w: %w", gofrHTTP.ErrorServiceUnavailable{}, ctxErr),
			cause:    ctxErr,
			reason:   reasonExtractorTimeout,
		}
	}

	if err != nil {
		return held{}, &extractionFailure{sentinel: err, cause: err, reason: reasonExtractorError}
	}

	list := make([]any, len(perms))
	for i, p := range perms {
		list[i] = p
	}

	return held{list: list, permissionsMode: true}, nil
}
