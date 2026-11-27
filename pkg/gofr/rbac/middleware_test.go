package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/internal/routemark"
	"gofr.dev/pkg/gofr/logging"
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/posts", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
	req.Header.Set("X-User-Role", "viewer")
	wrapped.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Forbidden: Access denied")
}

func TestExtractHeld(t *testing.T) {
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
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
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
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
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
			request:      httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc: "returns error when header configured but not present",
			config: &Config{
				RoleHeader: "X-User-Role",
			},
			request:      httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc:         "returns error when no role extraction configured",
			config:       &Config{},
			request:      httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody),
			expectedRole: "",
			expectError:  true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			h, err := extractHeld(tc.request, tc.config)
			role := h.str

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

func TestExtractHeld_JWTClaimPath(t *testing.T) {
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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)

			if tc.claims != nil {
				ctx := context.WithValue(req.Context(), middleware.JWTClaim, tc.claims)
				req = req.WithContext(ctx)
			}

			h, err := extractHeld(req, &Config{JWTClaimPath: tc.claimPath})
			role := h.str

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
			desc:        "the dotted walk wins over a top-level key of the same name",
			claims:      jwt.MapClaims{"a.b": "admin", "a": map[string]any{"b": "viewer"}},
			path:        "a.b",
			expected:    "viewer",
			expectError: false,
		},
		{
			desc:        "a top-level key with dots is used when the walk finds nothing",
			claims:      jwt.MapClaims{"https://example.com/roles": "admin"},
			path:        "https://example.com/roles",
			expected:    "admin",
			expectError: false,
		},
		{
			desc:        "a flat key with dots is read when the dotted walk fails on a non-object parent",
			claims:      jwt.MapClaims{"a": "x", "a.b": "admin"},
			path:        "a.b",
			expected:    "admin",
			expectError: false,
		},
		{
			desc:        "a flat key with dots is read when the dotted walk finds no parent",
			claims:      jwt.MapClaims{"realm_access.roles": []any{"admin"}},
			path:        "realm_access.roles",
			expected:    []any{"admin"},
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

func TestAuditLog_HeldCountIsAlwaysLogged(t *testing.T) {
	testCases := []struct {
		desc string
		log  AuditLog
		want string
	}{
		{desc: "a denial that held nothing logs held_count 0", log: AuditLog{Route: "/orders"}, want: `"held_count":0`},
		{desc: "a denial that held two logs held_count 2", log: AuditLog{Route: "/orders", HeldCount: 2}, want: `"held_count":2`},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			out, err := json.Marshal(tc.log)

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, string(out), tc.want, "TEST[%d], Failed.\n%s", i, tc.desc)
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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
			logAuditEvent(tc.logger, req, &AuditLog{Role: "admin", Route: "/api"}, tc.allowed)

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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)

			handleAuthError(w, req, tc.config, &denial{role: "admin", route: "/api", err: tc.err})

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
	warnLogs  []string
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

func (m *mockLogger) Warnf(format string, args ...any) {
	m.logs = append(m.logs, "WARNF")
	m.warnLogs = append(m.warnLogs, fmt.Sprintf(format, args...))
}

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
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "admin")

		// Setup role permissions
		config.Roles = []RoleDefinition{{Name: "admin", Permissions: []string{"admin:read"}}}
		require.NoError(t, config.processUnifiedConfig())

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
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "admin")

		// Setup role permissions
		config.Roles = []RoleDefinition{{Name: "admin", Permissions: []string{"admin:read"}}}
		require.NoError(t, config.processUnifiedConfig())

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
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api", http.NoBody)
		req.Header.Set("X-User-Role", "viewer") // Role without permission

		// Setup role permissions
		config.Roles = []RoleDefinition{{Name: "viewer", Permissions: []string{"viewer:read"}}}
		require.NoError(t, config.processUnifiedConfig())

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

type claimModeTestCase struct {
	desc       string
	config     Config
	claims     jwt.MapClaims // nil: no claims in the request context
	wantStatus int
	wantReason string // reason label on rbac_role_extraction_failures; empty: not incremented
	wantRole   string // AuditLog.Role and ErrorHandler role
	wantPerm   string // AuditLog.Permission
	wantCount  int    // AuditLog.HeldCount
	wantWarn   string
}

func claimModeEndpoints() []EndpointMapping {
	return []EndpointMapping{{Path: "/orders", Methods: []string{"GET"}, RequiredPermissions: []string{"orders:write"}}}
}

func claimModeTestCases() []claimModeTestCase {
	return append(rolesClaimModeTestCases(), permissionsClaimModeTestCases()...)
}

func rolesClaimModeTestCases() []claimModeTestCase {
	roles := []RoleDefinition{
		{Name: "admin", Permissions: []string{"orders:write"}},
		{Name: "viewer", Permissions: []string{"orders:read"}},
	}
	endpoints := claimModeEndpoints()

	return []claimModeTestCase{
		{
			desc:   "roles mode: array grants through any role, log names only the granting role",
			config: Config{JWTClaimPath: "roles", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"roles": []any{"viewer", "admin"}}, wantStatus: http.StatusOK, wantRole: "admin",
		},
		{
			desc:   "roles mode: array deny logs a count, never names",
			config: Config{JWTClaimPath: "roles", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"roles": []any{"viewer", "ghost"}}, wantStatus: http.StatusForbidden, wantCount: 2,
		},
		{
			desc:   "roles mode: roles[0] stays a single role, named on deny as before",
			config: Config{JWTClaimPath: "roles[0]", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"roles": []any{"viewer", "admin"}}, wantStatus: http.StatusForbidden,
			wantRole: "viewer",
		},
		{
			desc:   "roles mode: a whole-path top-level key wins over the dot walk",
			config: Config{JWTClaimPath: "https://example.com/roles", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"https://example.com/roles": []any{"admin"}}, wantStatus: http.StatusOK, wantRole: "admin",
		},
		{
			desc:   "roles mode: the dotted walk wins over a top-level key of the same name",
			config: Config{JWTClaimPath: "a.b", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"a.b": "admin", "a": map[string]any{"b": "viewer"}}, wantStatus: http.StatusForbidden,
			wantRole: "viewer",
		},
		{
			desc: "roles mode: an array of numbers is read as role names",
			config: Config{
				JWTClaimPath: "roles", Endpoints: endpoints,
				Roles: []RoleDefinition{{Name: "123", Permissions: []string{"orders:write"}}},
			},
			claims: jwt.MapClaims{"roles": []any{123.0}}, wantStatus: http.StatusOK, wantRole: "123",
		},
		{
			desc:   "roles mode: missing claim is 401",
			config: Config{JWTClaimPath: "roles", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"sub": "u1"}, wantStatus: http.StatusUnauthorized, wantReason: "missing_role",
		},
		{
			desc:       "roles mode: no claims in context is 401 and warns about call order",
			config:     Config{JWTClaimPath: "roles", Roles: roles, Endpoints: endpoints},
			wantStatus: http.StatusUnauthorized, wantReason: "no_jwt_claims", wantWarn: "call EnableOAuth before EnableRBAC",
		},
		{
			desc:   "roles mode: an object claim is 403 and warns with the path only",
			config: Config{JWTClaimPath: "roles", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"roles": map[string]any{"id": 12345.0}}, wantStatus: http.StatusForbidden,
			wantReason: "unreadable_claim", wantWarn: `"roles"`,
		},
		{
			desc: "roles mode: a number claim is read as a role name, as before multi-value claims",
			config: Config{
				JWTClaimPath: "role", Endpoints: endpoints,
				Roles: []RoleDefinition{{Name: "123", Permissions: []string{"orders:write"}}},
			},
			claims: jwt.MapClaims{"role": 123.0}, wantStatus: http.StatusOK, wantRole: "123",
		},
		{
			desc:   "roles mode: a boolean claim naming no role is 403 with that name",
			config: Config{JWTClaimPath: "role", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"role": true}, wantStatus: http.StatusForbidden, wantRole: "true",
		},
		{
			desc:   "roles mode: a null claim holds no role and is 403",
			config: Config{JWTClaimPath: "role", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"role": nil}, wantStatus: http.StatusForbidden,
		},
		{
			desc:   "roles mode: an empty claim is 401",
			config: Config{JWTClaimPath: "role", Roles: roles, Endpoints: endpoints},
			claims: jwt.MapClaims{"role": ""}, wantStatus: http.StatusUnauthorized, wantReason: "missing_role",
		},
	}
}

func permissionsClaimModeTestCases() []claimModeTestCase {
	endpoints := claimModeEndpoints()

	return []claimModeTestCase{
		{
			desc:       "permissions mode: scope string grants",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read orders:write"},
			wantStatus: http.StatusOK, wantPerm: "orders:write",
		},
		{
			desc:       "permissions mode: scope without the permission is 403 with a count",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read profile"},
			wantStatus: http.StatusForbidden, wantCount: 2,
		},
		{
			desc:       "permissions mode: missing scope holds nothing and is 403",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"sub": "u1"},
			wantStatus: http.StatusForbidden,
		},
		{
			desc:       "permissions mode: null scope holds nothing and is 403",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": nil},
			wantStatus: http.StatusForbidden,
		},
		{
			desc:       "permissions mode: empty scope holds nothing and is 403",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": ""},
			wantStatus: http.StatusForbidden,
		},
		{
			desc:       "permissions mode: empty scope array holds nothing and is 403",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": []any{}},
			wantStatus: http.StatusForbidden,
		},
		{
			desc:       "permissions mode: a number claim is 403 and warns with the path only",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": 12345.0},
			wantStatus: http.StatusForbidden, wantReason: "unreadable_claim", wantWarn: `"scope"`,
		},
	}
}

// serveClaimMode runs one request through the middleware and returns the response, the logger and
// the role and error the ErrorHandler received.
func serveClaimMode(t *testing.T, tc *claimModeTestCase) (
	w *httptest.ResponseRecorder, logger *mockLogger, handlerRole string, handlerErr error,
) {
	t.Helper()

	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	if tc.wantReason != "" {
		metrics.EXPECT().IncrementCounter(gomock.Any(), "rbac_role_extraction_failures", "reason", tc.wantReason)
	}

	logger = &mockLogger{}
	cfg := tc.config
	cfg.Logger, cfg.Metrics = logger, metrics

	handlerRole = "<not called>"
	cfg.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, role, _ string, err error) {
		handlerRole, handlerErr = role, err

		status := http.StatusForbidden
		if errors.Is(err, ErrRoleNotFound) {
			status = http.StatusUnauthorized
		}

		w.WriteHeader(status)
	}

	require.NoError(t, cfg.processUnifiedConfig())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/orders", http.NoBody)
	if tc.claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, tc.claims))
	}

	w = httptest.NewRecorder()
	Middleware(&cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	return w, logger, handlerRole, handlerErr
}

func TestMiddleware_ClaimModes(t *testing.T) {
	for i, tc := range claimModeTestCases() {
		t.Run(tc.desc, func(t *testing.T) {
			w, logger, handlerRole, handlerErr := serveClaimMode(t, &tc)

			assert.Equal(t, tc.wantStatus, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

			require.Len(t, logger.infoArgs, 1, "TEST[%d], Failed.\n%s", i, tc.desc)
			auditLog, ok := logger.infoArgs[0].(*AuditLog)
			require.True(t, ok, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantRole, auditLog.Role, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantPerm, auditLog.Permission, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantCount, auditLog.HeldCount, "TEST[%d], Failed.\n%s", i, tc.desc)

			if tc.wantStatus != http.StatusOK {
				assert.Equal(t, tc.wantRole, handlerRole, "TEST[%d], Failed.\n%s", i, tc.desc)

				// A custom ErrorHandler may compare with == or switch on the error, so it gets the bare sentinel
				// and never a wrapped cause.
				want := ErrAccessDenied
				if tc.wantStatus == http.StatusUnauthorized {
					want = ErrRoleNotFound
				}

				assert.Same(t, want, handlerErr, "TEST[%d], handler must get the bare sentinel, not %v.\n%s", i, handlerErr, tc.desc)
			}

			if tc.wantWarn == "" {
				assert.Empty(t, logger.warnLogs, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				require.Len(t, logger.warnLogs, 1, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Contains(t, logger.warnLogs[0], tc.wantWarn, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			for _, line := range logger.warnLogs {
				assert.NotContains(t, line, "12345", "TEST[%d], claim value leaked into a log.\n%s", i, tc.desc)
			}
		})
	}
}

func TestMiddleware_UnreadableClaimWarnsOnce(t *testing.T) {
	logger := &mockLogger{}
	cfg := &Config{
		JWTClaimPath: "roles",
		Roles:        []RoleDefinition{{Name: "admin", Permissions: []string{"a:b"}}},
		Endpoints:    []EndpointMapping{{Path: "/x", Methods: []string{"GET"}, RequiredPermissions: []string{"a:b"}}},
		Logger:       logger,
	}
	require.NoError(t, cfg.processUnifiedConfig())

	wrapped := Middleware(cfg)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for range 5 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, jwt.MapClaims{"roles": map[string]any{"id": 1.0}}))

		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	require.Len(t, logger.warnLogs, 1)
	assert.Contains(t, logger.warnLogs[0], `"roles"`)
}

func TestMiddleware_MissingClaimsWarnsOnce(t *testing.T) {
	logger := &mockLogger{}
	cfg := &Config{
		JWTClaimPath: "role",
		Roles:        []RoleDefinition{{Name: "admin", Permissions: []string{"a:b"}}},
		Endpoints: []EndpointMapping{
			{Path: "/x", Methods: []string{"GET"}, RequiredPermissions: []string{"a:b"}},
			{Path: "/.well-known/alive", Methods: []string{"GET"}, RequiredPermissions: []string{"a:b"}},
		},
		Logger: logger,
	}
	require.NoError(t, cfg.processUnifiedConfig())

	wrapped := Middleware(cfg)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	// OAuth never puts claims on /.well-known/* even when it runs first, so a request there must not
	// blame the call order, nor use up the one warning.
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/alive", http.NoBody))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, logger.warnLogs)

	for range 3 {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	require.Len(t, logger.warnLogs, 1)
	assert.Contains(t, logger.warnLogs[0], "call EnableOAuth before EnableRBAC")
}

func BenchmarkMiddleware(b *testing.B) {
	roleDefs := make([]RoleDefinition, 0, 10)
	roleNames := make([]any, 0, 10)
	scope := make([]string, 0, 10)

	for i := range 10 {
		name := fmt.Sprintf("role%d", i)
		roleDefs = append(roleDefs, RoleDefinition{Name: name, Permissions: []string{fmt.Sprintf("res%d:read", i)}})
		roleNames = append(roleNames, name)
		scope = append(scope, fmt.Sprintf("res%d:read", i))
	}

	endpoints := []EndpointMapping{{Path: "/r", Methods: []string{"GET"}, RequiredPermissions: []string{"res9:read"}}}

	benchmarks := []struct {
		name   string
		config *Config
		claims jwt.MapClaims
	}{
		{
			name:   "header mode, one role (baseline)",
			config: &Config{RoleHeader: "X-User-Role", Roles: roleDefs, Endpoints: endpoints},
		},
		{
			name:   "roles mode, 10 roles",
			config: &Config{JWTClaimPath: "roles", Roles: roleDefs, Endpoints: endpoints},
			claims: jwt.MapClaims{"roles": roleNames},
		},
		{
			name:   "permissions mode, 10-entry scope",
			config: &Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims: jwt.MapClaims{"scope": strings.Join(scope, " ")},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			if err := bm.config.processUnifiedConfig(); err != nil {
				b.Fatal(err)
			}

			wrapped := Middleware(bm.config)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			req := httptest.NewRequestWithContext(context.WithValue(b.Context(), middleware.JWTClaim, bm.claims),
				http.MethodGet, "/r", http.NoBody)
			req.Header.Set("X-User-Role", "role9")

			w := httptest.NewRecorder()

			b.ReportAllocs()

			for b.Loop() {
				wrapped.ServeHTTP(w, req)
			}
		})
	}
}

func TestAuditLog_PrettyPrint(t *testing.T) {
	testCases := []struct {
		desc     string
		log      AuditLog
		wantHeld string
	}{
		{desc: "role", log: AuditLog{Role: "admin", Permission: "orders:read", HeldCount: 3}, wantHeld: "[admin]"},
		{desc: "permission", log: AuditLog{Permission: "orders:read", HeldCount: 3}, wantHeld: "[orders:read]"},
		{desc: "held count", log: AuditLog{HeldCount: 3}, wantHeld: "[3 held]"},
		{desc: "nothing held", log: AuditLog{}, wantHeld: "[]"},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tc.log.CorrelationID, tc.log.Status, tc.log.Route = "corr-1", "DENIED", "/orders"

			var buf strings.Builder
			tc.log.PrettyPrint(&buf)

			out := buf.String()
			assert.Contains(t, out, "corr-1 DENIED", "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, out, "RBAC /orders "+tc.wantHeld, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractionFailure_Error(t *testing.T) {
	testCases := []struct {
		desc    string
		failure *extractionFailure
		want    string
	}{
		{desc: "401 sentinel hides the cause",
			failure: &extractionFailure{sentinel: ErrRoleNotFound, cause: errClaimPathNotFound, reason: reasonMissingRole},
			want:    ErrRoleNotFound.Error()},
		{desc: "403 sentinel hides the cause",
			failure: &extractionFailure{sentinel: ErrAccessDenied, cause: errUnreadableClaim, reason: reasonUnreadableClaim},
			want:    ErrAccessDenied.Error()},
	}

	for i, tc := range testCases {
		assert.Equal(t, tc.want, tc.failure.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestMiddleware_TraceRouteLabels(t *testing.T) {
	testCases := []struct {
		desc      string
		router    string
		path      string
		wantRoute string
		wantRule  string
		static    bool // serve a directory from "/", governed by a /{path:.*} rule
	}{
		{"mux: route and rule differ", gofrHTTP.MatcherMux, "/api/users/42", "/api/users/{id}", "/api/{path:.*}", false},
		{"trie: route and rule differ", gofrHTTP.MatcherTrie, "/api/users/42", "/api/users/{id}", "/api/{path:.*}", false},
		{"mux: no route and no rule", gofrHTTP.MatcherMux, "/nowhere", unknownRouteLabel, unknownRouteLabel, false},
		{"trie: no route and no rule", gofrHTTP.MatcherTrie, "/nowhere", unknownRouteLabel, unknownRouteLabel, false},
		{"mux: route at / is not the catch-all", gofrHTTP.MatcherMux, "/", "/", unknownRouteLabel, false},
		{"trie: route at / is not the catch-all", gofrHTTP.MatcherTrie, "/", "/", unknownRouteLabel, false},
		{"mux: static directory at / is not the catch-all", gofrHTTP.MatcherMux, "/a.txt", "/", "/{path:.*}", true},
		{"trie: static directory at / is not the catch-all", gofrHTTP.MatcherTrie, "/a.txt", "/", "/{path:.*}", true},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv(gofrHTTP.RouterEnvVar, tc.router)

			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

			endpoints := []EndpointMapping{{Path: "/api/{path:.*}", Methods: []string{http.MethodGet}, Public: true}}
			if tc.static {
				endpoints = append(endpoints, EndpointMapping{Path: "/{path:.*}", Methods: []string{http.MethodGet}, Public: true})
			}

			config := newTestConfig(t, endpoints, nil)
			config.Tracer = provider.Tracer("test")

			ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

			router := gofrHTTP.NewRouter()
			router.Use(Middleware(config))
			router.Add(http.MethodGet, "/", ok)
			router.Add(http.MethodGet, "/api/users/{id}", ok)

			if tc.static {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600))
				router.AddStaticFiles(logging.NewMockLogger(logging.ERROR), "/", dir)
			}

			routemark.MarkCatchAll(router.PathPrefix("/")).Handler(ok)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if tc.static {
				require.Equal(t, http.StatusOK, rec.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			spans := recorder.Ended()
			require.Len(t, spans, 1, "TEST[%d], Failed.\n%s", i, tc.desc)

			attrs := make(map[string]string)
			for _, kv := range spans[0].Attributes() {
				attrs[string(kv.Key)] = kv.Value.String()
			}

			assert.Equal(t, tc.wantRoute, attrs["http.route"], "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantRule, attrs["rbac.rule"], "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestMiddleware_LiteralPatternDoesNotBypassProtectedHandler(t *testing.T) {
	// A request whose URL-encoded path matches a pattern rule's literal string (e.g. "/reports/%7Bid:%5B0-9%5D+%7D")
	// must not bypass authentication when a narrower protected route covers it.
	endpoints := []EndpointMapping{
		{Path: "/reports/{id:[0-9]+}", Methods: []string{"GET"}, Public: true},
		{Path: "/reports/{name}", Methods: []string{"GET"}, RequiredPermissions: []string{"reports:read"}},
	}
	roles := []RoleDefinition{
		{Name: "admin", Permissions: []string{"reports:read"}},
	}
	config := newTestConfig(t, endpoints, roles)

	reached := false
	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))

	// An unauthenticated request with URL-encoded pattern text:
	// Path decodes to "/reports/{id:[0-9]+}", which does NOT match the public regex "[0-9]+".
	// It matches "/reports/{name}" and must be rejected with 401 Unauthorized.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reports/%7Bid:%5B0-9%5D+%7D", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.False(t, reached, "unauthenticated request matching protected route must not reach handler")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// With proper role, it should be allowed
	reached = false

	reqAuth := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reports/%7Bid:%5B0-9%5D+%7D", http.NoBody)
	reqAuth.Header.Set("X-User-Role", "admin")

	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)

	assert.True(t, reached, "authenticated request must reach handler")
	assert.Equal(t, http.StatusOK, recAuth.Code)
}

func TestMiddleware_ProtectedOnlyLiteralPatternRetainsUnauthorized(t *testing.T) {
	// An unauthenticated request whose path literally equals a protected pattern rule (e.g. "/items/%7Bid:%5B0-9%5D+%7D")
	// must match the protected exact pattern key and return HTTP 401 Unauthorized, not 404.
	endpoints := []EndpointMapping{
		{Path: "/items/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"items:read"}},
	}
	roles := []RoleDefinition{
		{Name: "admin", Permissions: []string{"items:read"}},
	}
	config := newTestConfig(t, endpoints, roles)

	reached := false
	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))

	// Unauthenticated request with URL-encoded literal pattern text matches the protected pattern key -> 401
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/%7Bid:%5B0-9%5D+%7D", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.False(t, reached, "unauthenticated request must not reach handler")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// With proper role, it should be allowed
	reached = false

	reqAuth := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/%7Bid:%5B0-9%5D+%7D", http.NoBody)
	reqAuth.Header.Set("X-User-Role", "admin")

	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)

	assert.True(t, reached, "authenticated request must reach handler")
	assert.Equal(t, http.StatusOK, recAuth.Code)
}
