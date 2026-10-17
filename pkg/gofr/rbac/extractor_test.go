package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

var errStoreDown = errors.New("store down")

// statusError is an application error that picks its own status code.
type statusError struct{ code int }

func (statusError) Error() string     { return "status error" }
func (e statusError) StatusCode() int { return e.code }

// staticExtractor returns perms and err whatever the request.
func staticExtractor(perms []string, err error) Extractor {
	return func(*container.Container, *http.Request) ([]string, error) { return perms, err }
}

// withTestExtractor sets an extractor that grants nothing.
func withTestExtractor() Option {
	return WithExtractor(staticExtractor(nil, nil), nil, 0)
}

// blockingExtractor waits for the request context to end, the way an extractor that passes
// r.Context() to a slow store does.
func blockingExtractor(_ *container.Container, r *http.Request) ([]string, error) {
	<-r.Context().Done()

	return nil, r.Context().Err()
}

// sleepingExtractor ignores the context and grants after d.
func sleepingExtractor(d time.Duration) Extractor {
	return func(*container.Container, *http.Request) ([]string, error) {
		time.Sleep(d)

		return []string{"orders:read"}, nil
	}
}

type extractorTestCase struct {
	desc         string
	extractor    Extractor
	timeout      time.Duration
	path         string
	cancelReq    bool // the request context is canceled before RBAC runs
	errorHandler bool // a custom ErrorHandler is set

	wantStatus  int
	wantCalled  bool   // the extractor was called
	wantServed  bool   // the next handler was called
	wantReason  string // the reason on the extraction failures metric, if counted
	wantPerm    string // the permission the audit log names on success
	wantCount   int    // the held count the audit log names on a denial after extraction
	wantErrIs   error  // what the ErrorHandler's err must match, when errorHandler is set
	wantErrSame bool   // the ErrorHandler gets wantErrIs itself, not a wrapper
}

func extractorTestCases() []extractorTestCase {
	return []extractorTestCase{
		{desc: "a returned permission that is required grants",
			extractor:  staticExtractor([]string{"orders:write", "orders:read"}, nil),
			wantStatus: http.StatusOK, wantCalled: true, wantServed: true, wantPerm: "orders:read"},
		{desc: "permissions without a required one are denied",
			extractor:  staticExtractor([]string{"orders:write"}, nil),
			wantStatus: http.StatusForbidden, wantCalled: true, wantCount: 1},
		{desc: "an empty slice holds nothing",
			extractor:  staticExtractor([]string{}, nil),
			wantStatus: http.StatusForbidden, wantCalled: true},
		{desc: "nil holds nothing",
			extractor:  staticExtractor(nil, nil),
			wantStatus: http.StatusForbidden, wantCalled: true},
		{desc: "empty entries are skipped",
			extractor:  staticExtractor([]string{"", "orders:read"}, nil),
			wantStatus: http.StatusOK, wantCalled: true, wantServed: true, wantPerm: "orders:read"},
		{desc: "a plain error is a 403, even with permissions returned",
			extractor:  staticExtractor([]string{"orders:read"}, errStoreDown),
			wantStatus: http.StatusForbidden, wantCalled: true, wantReason: reasonExtractorError},
		{desc: "an error with a status code sets the status",
			extractor:  staticExtractor(nil, gofrHTTP.ErrorServiceUnavailable{Dependency: "store"}),
			wantStatus: http.StatusServiceUnavailable, wantCalled: true, wantReason: reasonExtractorError},
		{desc: "a wrapped error with a status code sets the status",
			extractor:  staticExtractor(nil, errors.Join(errStoreDown, statusError{code: http.StatusTooManyRequests})),
			wantStatus: http.StatusTooManyRequests, wantCalled: true, wantReason: reasonExtractorError},
		{desc: "an error with a success status code is still a 403",
			extractor:  staticExtractor([]string{"orders:read"}, statusError{code: http.StatusOK}),
			wantStatus: http.StatusForbidden, wantCalled: true, wantReason: reasonExtractorError},
		{desc: "ErrRoleNotFound is a 401",
			extractor:  staticExtractor(nil, ErrRoleNotFound),
			wantStatus: http.StatusUnauthorized, wantCalled: true, wantReason: reasonExtractorError},
		{desc: "an extractor that waits past the timeout is a 503",
			extractor: blockingExtractor, timeout: 10 * time.Millisecond,
			wantStatus: http.StatusServiceUnavailable, wantCalled: true, wantReason: reasonExtractorTimeout},
		{desc: "an extractor that ignores the deadline and grants late is still denied",
			extractor: sleepingExtractor(30 * time.Millisecond), timeout: 5 * time.Millisecond,
			wantStatus: http.StatusServiceUnavailable, wantCalled: true, wantReason: reasonExtractorTimeout},
		{desc: "a canceled request is denied even when the extractor grants",
			extractor: staticExtractor([]string{"orders:read"}, nil), cancelReq: true,
			wantStatus: http.StatusServiceUnavailable, wantCalled: true, wantReason: reasonExtractorTimeout},
		{desc: "a public route never calls the extractor",
			extractor: staticExtractor(nil, errStoreDown), path: "/health",
			wantStatus: http.StatusOK, wantServed: true},
		{desc: "a route no rule matches never calls the extractor",
			extractor: staticExtractor(nil, errStoreDown), path: "/unruled",
			wantStatus: http.StatusOK, wantServed: true},
		{desc: "the ErrorHandler gets the extractor's error itself",
			extractor: staticExtractor(nil, errStoreDown), errorHandler: true,
			wantStatus: http.StatusTeapot, wantCalled: true, wantReason: reasonExtractorError,
			wantErrIs: errStoreDown, wantErrSame: true},
		{desc: "the ErrorHandler gets a deadline error on a timeout",
			extractor: blockingExtractor, timeout: 10 * time.Millisecond, errorHandler: true,
			wantStatus: http.StatusTeapot, wantCalled: true, wantReason: reasonExtractorTimeout,
			wantErrIs: context.DeadlineExceeded},
		{desc: "the ErrorHandler gets ErrAccessDenied when nothing granted",
			extractor: staticExtractor([]string{"orders:write"}, nil), errorHandler: true,
			wantStatus: http.StatusTeapot, wantCalled: true, wantCount: 1,
			wantErrIs: ErrAccessDenied, wantErrSame: true},
	}
}

func extractorEndpoints() []EndpointMapping {
	return []EndpointMapping{
		{Path: "/health", Methods: []string{"GET"}, Public: true},
		{Path: "/orders", Methods: []string{"GET"}, RequiredPermissions: []string{"orders:read"}},
	}
}

type extractorResult struct {
	w          *httptest.ResponseRecorder
	logger     *mockLogger
	called     bool
	served     bool
	handlerErr error
}

func serveExtractor(t *testing.T, tc *extractorTestCase) extractorResult {
	t.Helper()

	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	if tc.wantReason != "" {
		metrics.EXPECT().IncrementCounter(gomock.Any(), "rbac_role_extraction_failures", "reason", tc.wantReason)
	}

	c := &container.Container{}

	var res extractorResult

	cfg := Config{Endpoints: extractorEndpoints()}
	WithExtractor(func(got *container.Container, r *http.Request) ([]string, error) {
		res.called = true

		assert.Same(t, c, got, "the extractor must get the container it was set up with")

		_, hasDeadline := r.Context().Deadline()
		assert.True(t, hasDeadline, "the extractor must run under a deadline")

		return tc.extractor(got, r)
	}, c, tc.timeout)(&cfg)

	res.logger = &mockLogger{}
	cfg.Logger, cfg.Metrics = res.logger, metrics

	if tc.errorHandler {
		cfg.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _, _ string, err error) {
			res.handlerErr = err

			w.WriteHeader(http.StatusTeapot)
		}
	}

	require.NoError(t, cfg.validate())
	require.NoError(t, cfg.processUnifiedConfig())

	path := tc.path
	if path == "" {
		path = "/orders"
	}

	ctx := t.Context()

	if tc.cancelReq {
		var cancel context.CancelFunc

		ctx, cancel = context.WithCancel(ctx)
		cancel()
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	res.w = httptest.NewRecorder()

	Middleware(&cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		res.served = true

		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(res.w, req)

	return res
}

func TestMiddleware_Extractor(t *testing.T) {
	for i, tc := range extractorTestCases() {
		t.Run(tc.desc, func(t *testing.T) {
			res := serveExtractor(t, &tc)

			assert.Equal(t, tc.wantStatus, res.w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantCalled, res.called, "TEST[%d], extractor called.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantServed, res.served, "TEST[%d], handler served.\n%s", i, tc.desc)

			// The client sees the status text only, never the extractor's error message.
			assert.NotContains(t, res.w.Body.String(), "store", "TEST[%d], error leaked.\n%s", i, tc.desc)

			if tc.wantCalled {
				require.Len(t, res.logger.infoArgs, 1, "TEST[%d], Failed.\n%s", i, tc.desc)
				auditLog, ok := res.logger.infoArgs[0].(*AuditLog)
				require.True(t, ok, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Empty(t, auditLog.Role, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.wantPerm, auditLog.Permission, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.wantCount, auditLog.HeldCount, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			if tc.wantErrIs == nil {
				return
			}

			require.ErrorIs(t, res.handlerErr, tc.wantErrIs, "TEST[%d], Failed.\n%s", i, tc.desc)

			if tc.wantErrSame {
				assert.Same(t, tc.wantErrIs, res.handlerErr, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestMiddleware_ExtractorTimeoutWarnsOnce(t *testing.T) {
	logger := &mockLogger{}

	var calls atomic.Int32

	cfg := &Config{Endpoints: extractorEndpoints(), Logger: logger}
	WithExtractor(func(c *container.Container, r *http.Request) ([]string, error) {
		calls.Add(1)

		return blockingExtractor(c, r)
	}, nil, 5*time.Millisecond)(cfg)
	require.NoError(t, cfg.processUnifiedConfig())

	wrapped := Middleware(cfg)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for range 3 {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/orders", http.NoBody))
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	}

	assert.Equal(t, int32(3), calls.Load())
	require.Len(t, logger.warnLogs, 1)
	assert.Contains(t, logger.warnLogs[0], "extractor did not return within 5ms")
}

func TestLoadPermissions_ExtractorMode(t *testing.T) {
	const endpoints = `"endpoints": [{"path": "/orders", "methods": ["GET"], "requiredPermissions": ["orders:read"]}]`

	const startLine = "RBAC enabled: mode=extractor, source=application"

	testCases := []struct {
		desc    string
		content string
		opt     Option
		wantErr error
		warn    string
	}{
		{desc: "an extractor runs in extractor mode",
			content: `{` + endpoints + `}`, opt: withTestExtractor()},
		{desc: "an extractor with jwtClaimPath is rejected",
			content: `{"jwtClaimPath": "roles", ` + endpoints + `}`, opt: withTestExtractor(),
			wantErr: errExtractorWithClaimPath},
		{desc: "an extractor with permissionsClaimPath is rejected",
			content: `{"permissionsClaimPath": "scope", ` + endpoints + `}`, opt: withTestExtractor(),
			wantErr: errExtractorWithClaimPath},
		{desc: "a nil extractor is rejected",
			content: `{` + endpoints + `}`, opt: WithExtractor(nil, nil, 0),
			wantErr: errNilExtractor},
		{desc: "an extractor with a roles section warns that the roles are not consulted",
			content: `{"roles": [{"name": "a", "permissions": ["orders:read"]}], ` + endpoints + `}`,
			opt:     withTestExtractor(), warn: "roles section is ignored"},
		{desc: "an extractor with roleHeader warns that the header is ignored",
			content: `{"roleHeader": "X-User-Role", ` + endpoints + `}`, opt: withTestExtractor(),
			warn: "roleHeader is ignored"},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rbac.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0600))

			logger := &mockLogger{}

			config, err := LoadPermissions(path, logger, nil, nil, tc.opt)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, logger.infoLogs, startLine, "TEST[%d], Failed.\n%s", i, tc.desc)

			if tc.warn == "" {
				assert.Empty(t, logger.warnLogs, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.Len(t, logger.warnLogs, 1, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Contains(t, logger.warnLogs[0], tc.warn, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestWithExtractor(t *testing.T) {
	testCases := []struct {
		desc        string
		timeout     time.Duration
		wantTimeout time.Duration
	}{
		{desc: "a positive timeout is kept", timeout: time.Second, wantTimeout: time.Second},
		{desc: "zero uses the default", timeout: 0, wantTimeout: defaultExtractorTimeout},
		{desc: "a negative timeout uses the default", timeout: -time.Second, wantTimeout: defaultExtractorTimeout},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &container.Container{}

			var cfg Config
			WithExtractor(staticExtractor(nil, nil), c, tc.timeout)(&cfg)

			assert.NotNil(t, cfg.extractor, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Same(t, c, cfg.container, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantTimeout, cfg.extractorTimeout, "TEST[%d], Failed.\n%s", i, tc.desc)

			mode, source := cfg.mode()
			assert.Equal(t, modeExtractor, mode, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, "application", source, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
