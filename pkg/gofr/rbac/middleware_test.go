package rbac

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/http/middleware"
)

func TestMiddleware_NilConfig(t *testing.T) {
	middlewareFunc := Middleware(nil)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestMiddleware_PublicEndpoint(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/health", Methods: []string{"GET"}, Public: true},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	middlewareFunc := Middleware(config)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestMiddleware_NoEndpointMatch(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	middlewareFunc := Middleware(config)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", http.NoBody)
	wrapped.ServeHTTP(w, req)

	// Routes not in RBAC config are handled by normal route matching
	// So unmatched endpoints should be allowed through
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestMiddleware_RoleNotFound(t *testing.T) {
	config := &Config{
		RoleHeader: "X-User-Role",
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"admin:read", "admin:write"}},
		},
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	middlewareFunc := Middleware(config)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Unauthorized: Missing or invalid role")
}

func TestMiddleware_ValidRoleAndPermission(t *testing.T) {
	config := &Config{
		RoleHeader: "X-User-Role",
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"admin:read", "admin:write"}},
		},
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	middlewareFunc := Middleware(config)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
	req.Header.Set("X-User-Role", "admin")
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestMiddleware_InvalidPermission(t *testing.T) {
	config := &Config{
		RoleHeader: "X-User-Role",
		Roles: []RoleDefinition{
			{Name: "viewer", Permissions: []string{"users:read"}},
		},
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"users:write"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	middlewareFunc := Middleware(config)
	require.NotNil(t, middlewareFunc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := middlewareFunc(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
	req.Header.Set("X-User-Role", "viewer")
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Forbidden: Access denied")
}

func TestExtractRole(t *testing.T) {
	testCases := []struct {
		desc         string
		config       *Config
		request      *http.Request
		expectedRole string
		expectError  bool
	}{
		{
			desc: "extracts role from header",
			config: &Config{
				RoleHeader: "X-User-Role",
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
				req.Header.Set("X-User-Role", "admin")
				return req
			}(),
			expectedRole: "admin",
			expectError:  false,
		},
		{
			desc: "extracts role from JWT when both configured",
			config: &Config{
				RoleHeader:   "X-User-Role",
				JWTClaimPath: "role",
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
				req.Header.Set("X-User-Role", "viewer")
				claims := jwt.MapClaims{"role": "admin"}
				ctx := context.WithValue(req.Context(), middleware.JWTClaim, claims)
				return req.WithContext(ctx)
			}(),
			expectedRole: "admin",
			expectError:  false,
		},
		{
			desc: "returns error when JWT configured but claims not found",
			config: &Config{
				JWTClaimPath: "role",
			},
			request:      httptest.NewRequest(http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc: "returns error when header configured but not present",
			config: &Config{
				RoleHeader: "X-User-Role",
			},
			request:      httptest.NewRequest(http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc:         "returns error when no role extraction configured",
			config:       &Config{},
			request:      httptest.NewRequest(http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			role, err := extractRole(tc.request, tc.config)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Empty(t, role, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expectedRole, role, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractRoleFromJWT(t *testing.T) {
	testCases := []struct {
		desc         string
		claimPath    string
		claims       jwt.MapClaims
		expectedRole string
		expectError  bool
	}{
		{
			desc:      "extracts role from simple claim",
			claimPath: "role",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			expectedRole: "admin",
			expectError:  false,
		},
		{
			desc:      "extracts role from array claim",
			claimPath: "roles[0]",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user"},
			},
			expectedRole: "admin",
			expectError:  false,
		},
		{
			desc:      "extracts role from nested claim",
			claimPath: "permissions.role",
			claims: jwt.MapClaims{
				"permissions": map[string]any{
					"role": "admin",
				},
			},
			expectedRole: "admin",
			expectError:  false,
		},
		{
			desc:         "returns error when claims not in context",
			claimPath:    "role",
			claims:       nil,
			expectedRole: "",
			expectError:  true,
		},
		{
			desc:      "returns error when claim path not found",
			claimPath: "nonexistent",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			expectedRole: "",
			expectError:  true,
		},
		{
			desc:      "converts non-string role to string",
			claimPath: "role",
			claims: jwt.MapClaims{
				"role": 123,
			},
			expectedRole: "123",
			expectError:  false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)

			if tc.claims != nil {
				ctx := context.WithValue(req.Context(), middleware.JWTClaim, tc.claims)
				req = req.WithContext(ctx)
			}

			role, err := extractRoleFromJWT(req, tc.claimPath)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Empty(t, role, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expectedRole, role, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractClaimValue(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "extracts simple claim",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			path:        "role",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "extracts array claim",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user"},
			},
			path:        "roles[0]",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "extracts nested claim",
			claims: jwt.MapClaims{
				"permissions": map[string]any{
					"role": "admin",
				},
			},
			path:        "permissions.role",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "returns error for empty path",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			path:        "",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for non-existent claim",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			path:        "nonexistent",
			expected:    nil,
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := extractClaimValue(tc.claims, tc.path)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractArrayClaim_Basic(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "extracts first element from array",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user"},
			},
			path:        "roles[0]",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "extracts second element from array",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user"},
			},
			path:        "roles[1]",
			expected:    "user",
			expectError: false,
		},
		{
			desc: "handles array with mixed types",
			claims: jwt.MapClaims{
				"roles": []any{123, "admin", true},
			},
			path:        "roles[0]",
			expected:    123,
			expectError: false,
		},
	}

	runExtractArrayClaimTests(t, testCases)
}

func TestExtractArrayClaim_Errors(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "returns error for invalid array notation",
			claims: jwt.MapClaims{
				"roles": []any{"admin"},
			},
			path:        "roles[",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for non-existent key",
			claims: jwt.MapClaims{
				"other": []any{"value"},
			},
			path:        "roles[0]",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error when value is not array",
			claims: jwt.MapClaims{
				"roles": "not an array",
			},
			path:        "roles[0]",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for out of bounds index",
			claims: jwt.MapClaims{
				"roles": []any{"admin"},
			},
			path:        "roles[5]",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for negative index",
			claims: jwt.MapClaims{
				"roles": []any{"admin"},
			},
			path:        "roles[-1]",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "handles empty array",
			claims: jwt.MapClaims{
				"roles": []any{},
			},
			path:        "roles[0]",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "handles nil value in claims",
			claims: jwt.MapClaims{
				"roles": nil,
			},
			path:        "roles[0]",
			expected:    nil,
			expectError: true,
		},
	}

	runExtractArrayClaimTests(t, testCases)
}

func TestExtractArrayClaim_EdgeCases(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "handles array with nil elements",
			claims: jwt.MapClaims{
				"roles": []any{nil, "admin"},
			},
			path:        "roles[0]",
			expected:    nil,
			expectError: false,
		},
	}

	runExtractArrayClaimTests(t, testCases)
}

func runExtractArrayClaimTests(t *testing.T, testCases []struct {
	desc        string
	claims      jwt.MapClaims
	path        string
	expected    any
	expectError bool
}) {
	t.Helper()

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			idx := 0

			for j, c := range tc.path {
				if c == '[' {
					idx = j
					break
				}
			}

			result, err := extractArrayClaim(tc.claims, tc.path, idx)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractNestedClaim_Basic(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "extracts nested claim",
			claims: jwt.MapClaims{
				"permissions": map[string]any{
					"role": "admin",
				},
			},
			path:        "permissions.role",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "extracts deeply nested claim",
			claims: jwt.MapClaims{
				"user": map[string]any{
					"profile": map[string]any{
						"role": "admin",
					},
				},
			},
			path:        "user.profile.role",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "handles array in nested structure",
			claims: jwt.MapClaims{
				"data": map[string]any{
					"roles": []any{"admin", "user"},
				},
			},
			path:        "data.roles",
			expected:    []any{"admin", "user"},
			expectError: false,
		},
	}

	runExtractNestedClaimTests(t, testCases)
}

func TestExtractNestedClaim_Errors(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "returns error for non-existent path",
			claims: jwt.MapClaims{
				"permissions": map[string]any{
					"role": "admin",
				},
			},
			path:        "permissions.nonexistent",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for invalid structure",
			claims: jwt.MapClaims{
				"permissions": "not a map",
			},
			path:        "permissions.role",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error when intermediate path is nil",
			claims: jwt.MapClaims{
				"permissions": nil,
			},
			path:        "permissions.role",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "handles empty nested map",
			claims: jwt.MapClaims{
				"permissions": map[string]any{},
			},
			path:        "permissions.role",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "handles deeply nested nil intermediate value",
			claims: jwt.MapClaims{
				"user": map[string]any{
					"profile": nil,
				},
			},
			path:        "user.profile.role",
			expected:    nil,
			expectError: true,
		},
	}

	runExtractNestedClaimTests(t, testCases)
}

func TestExtractNestedClaim_EdgeCases(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "handles nil value in nested map",
			claims: jwt.MapClaims{
				"permissions": map[string]any{
					"role": nil,
				},
			},
			path:        "permissions.role",
			expected:    nil,
			expectError: false,
		},
	}

	runExtractNestedClaimTests(t, testCases)
}

func runExtractNestedClaimTests(t *testing.T, testCases []struct {
	desc        string
	claims      jwt.MapClaims
	path        string
	expected    any
	expectError bool
}) {
	t.Helper()

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := extractNestedClaim(tc.claims, tc.path)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestLogAuditEvent(t *testing.T) {
	testCases := []struct {
		desc     string
		logger   datasource.Logger
		allowed  bool
		expected int
	}{
		{
			desc:     "logs allowed event",
			logger:   &mockLogger{logs: []string{}},
			allowed:  true,
			expected: 1,
		},
		{
			desc:     "logs denied event",
			logger:   &mockLogger{logs: []string{}},
			allowed:  false,
			expected: 1,
		},
		{
			desc:     "does not log when logger is nil",
			logger:   nil,
			allowed:  true,
			expected: 0,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
			logAuditEvent(tc.logger, req, "admin", "/api", tc.allowed)

			if tc.logger != nil {
				mockLog := tc.logger.(*mockLogger)
				assert.GreaterOrEqual(t, len(mockLog.logs), tc.expected, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestHandleAuthError(t *testing.T) {
	testCases := []struct {
		desc           string
		config         *Config
		err            error
		expectedStatus int
		expectedBody   string
		customHandler  bool
	}{
		{
			desc: "handles ErrRoleNotFound with default handler",
			config: &Config{
				Logger: &mockLogger{logs: []string{}},
			},
			err:            ErrRoleNotFound,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Missing or invalid role",
			customHandler:  false,
		},
		{
			desc: "handles ErrAccessDenied with default handler",
			config: &Config{
				Logger: &mockLogger{logs: []string{}},
			},
			err:            ErrAccessDenied,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden: Access denied",
			customHandler:  false,
		},
		{
			desc: "uses custom error handler when provided",
			config: &Config{
				Logger: &mockLogger{logs: []string{}},
				ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _, _ string, _ error) {
					w.WriteHeader(http.StatusTeapot)
					_, _ = w.Write([]byte("Custom error"))
				},
			},
			err:            ErrAccessDenied,
			expectedStatus: http.StatusTeapot,
			expectedBody:   "Custom error",
			customHandler:  true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)

			handleAuthError(w, req, tc.config, "admin", "/api", tc.err)

			assert.Equal(t, tc.expectedStatus, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, w.Body.String(), tc.expectedBody, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// mockLogger implements the datasource.Logger interface for testing.
type mockLogger struct {
	errorLogs []string
	infoLogs  []string // Capture actual log messages
	logs      []string
	infoArgs  []any // Capture structured log arguments (used for both Info and Debug)
}

func (m *mockLogger) Debug(args ...any) {
	m.logs = append(m.logs, "DEBUG")
	if len(args) > 0 {
		m.infoArgs = append(m.infoArgs, args...)
	}
}
func (m *mockLogger) Debugf(_ string, _ ...any) { m.logs = append(m.logs, "DEBUGF") }
func (m *mockLogger) Info(args ...any) {
	m.logs = append(m.logs, "INFO")
	if len(args) > 0 {
		m.infoArgs = append(m.infoArgs, args...)
	}
}
func (m *mockLogger) Infof(format string, args ...any) {
	m.logs = append(m.logs, "INFOF")
	m.infoLogs = append(m.infoLogs, fmt.Sprintf(format, args...))
}

func (m *mockLogger) Error(_ ...any) { m.logs = append(m.logs, "ERROR") }
func (m *mockLogger) Errorf(format string, args ...any) {
	m.logs = append(m.logs, "ERRORF")
	m.errorLogs = append(m.errorLogs, fmt.Sprintf(format, args...))
}

func (m *mockLogger) Warn(_ ...any) { m.logs = append(m.logs, "WARN") }

func (m *mockLogger) Warnf(_ string, _ ...any) { m.logs = append(m.logs, "WARNF") }

func TestMiddleware_WithTracing(t *testing.T) {
	t.Run("starts tracing when tracer is available", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}},
			},
			RoleHeader: "X-User-Role",
			Tracer:     noop.NewTracerProvider().Tracer("test"),
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		middlewareFunc := Middleware(config)
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := middlewareFunc(handler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "admin")

		// Setup role permissions
		config.rolePermissionsMap = map[string][]string{
			"admin": {"admin:read"},
		}

		wrapped.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestMiddleware_RoleInAuditLogs(t *testing.T) {
	t.Run("role is included in audit logs", func(t *testing.T) {
		mockLog := &mockLogger{
			logs:     []string{},
			infoLogs: []string{},
			infoArgs: []any{},
		}

		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}},
			},
			RoleHeader: "X-User-Role",
			Logger:     mockLog,
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		middlewareFunc := Middleware(config)
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := middlewareFunc(handler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "admin")

		// Setup role permissions
		config.rolePermissionsMap = map[string][]string{
			"admin": {"admin:read"},
		}

		wrapped.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Verify audit log contains role
		assert.NotEmpty(t, mockLog.logs, "audit log should be written")
		// Verify that Debug was called (structured logging)
		assert.Contains(t, mockLog.logs, "DEBUG", "audit log should be written via Debug")
		// Verify structured log contains AuditLog
		assert.NotEmpty(t, mockLog.infoArgs, "audit log struct should be captured")
		auditLog, ok := mockLog.infoArgs[0].(*AuditLog)
		require.True(t, ok, "audit log should be AuditLog struct")
		assert.Equal(t, "admin", auditLog.Role, "audit log should contain role")
		assert.Equal(t, "ACC", auditLog.Status, "audit log should have ACC status")
		assert.Equal(t, "GET", auditLog.Method, "audit log should contain method")
		assert.Equal(t, "/api", auditLog.Route, "audit log should contain route")
		assert.NotEmpty(t, auditLog.CorrelationID, "audit log should contain correlation ID")
	})

	t.Run("role is included in audit logs for denied requests", func(t *testing.T) {
		mockLog := &mockLogger{
			logs:     []string{},
			infoLogs: []string{},
			infoArgs: []any{},
		}

		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}},
			},
			RoleHeader: "X-User-Role",
			Logger:     mockLog,
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		middlewareFunc := Middleware(config)
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := middlewareFunc(handler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "viewer") // Role without permission

		// Setup role permissions
		config.rolePermissionsMap = map[string][]string{
			"viewer": {"viewer:read"}, // Different permission
		}

		wrapped.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		// Verify audit log contains role
		assert.NotEmpty(t, mockLog.logs, "audit log should be written")
		// Verify that Debug was called (structured logging)
		assert.Contains(t, mockLog.logs, "DEBUG", "audit log should be written via Debug")
		// Verify structured log contains AuditLog
		assert.NotEmpty(t, mockLog.infoArgs, "audit log struct should be captured")
		auditLog, ok := mockLog.infoArgs[0].(*AuditLog)
		require.True(t, ok, "audit log should be AuditLog struct")
		assert.Equal(t, "viewer", auditLog.Role, "audit log should contain role")
		assert.Equal(t, "REJ", auditLog.Status, "audit log should have REJ status")
		assert.Equal(t, "GET", auditLog.Method, "audit log should contain method")
		assert.Equal(t, "/api", auditLog.Route, "audit log should contain route")
	})
}

func TestSanitizeErrorForTrace(t *testing.T) {
	t.Run("sanitizes known errors", func(t *testing.T) {
		err := ErrRoleNotFound
		sanitized := sanitizeErrorForTrace(err)
		assert.Equal(t, ErrRoleNotFound, sanitized, "known errors should be returned as-is")
	})

	t.Run("sanitizes access denied errors", func(t *testing.T) {
		err := ErrAccessDenied
		sanitized := sanitizeErrorForTrace(err)
		assert.Equal(t, ErrAccessDenied, sanitized, "known errors should be returned as-is")
	})

	t.Run("sanitizes unknown errors", func(t *testing.T) {
		//nolint:err113 // Test intentionally uses dynamic errors to verify sanitization
		testErr := fmt.Errorf("internal system error: database connection failed at 192.168.1.1")
		sanitized := sanitizeErrorForTrace(testErr)
		// Unknown errors should be sanitized to generic message
		assert.Equal(t, "authorization error", sanitized.Error(), "unknown errors should be sanitized")
		assert.NotContains(t, sanitized.Error(), "192.168.1.1", "sensitive information should be removed")
		assert.NotContains(t, sanitized.Error(), "database connection", "internal details should be removed")
	})

	t.Run("sanitizes wrapped errors", func(t *testing.T) {
		//nolint:err113 // Test intentionally uses dynamic errors to verify sanitization
		testErr := fmt.Errorf("internal system error: secret key exposed")
		err := fmt.Errorf("wrapped error: %w", testErr)
		sanitized := sanitizeErrorForTrace(err)
		// Wrapped unknown errors should be sanitized
		assert.Equal(t, "authorization error", sanitized.Error(), "wrapped unknown errors should be sanitized")
		assert.NotContains(t, sanitized.Error(), "secret key", "sensitive information should be removed")
	})
}
