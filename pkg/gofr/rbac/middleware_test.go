package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/http/middleware"
)

func TestMiddleware(t *testing.T) {
	testCases := []struct {
		desc           string
		config         *Config
		request        *http.Request
		expectedStatus int
		expectedBody   string
	}{
		{
			desc:   "allows request when config is nil",
			config: nil,
			request: httptest.NewRequest("GET", "/api", nil),
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			desc: "allows public endpoint",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/health", Methods: []string{"GET"}, Public: true},
				},
			},
			request: httptest.NewRequest("GET", "/health", nil),
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			desc: "denies request when no endpoint match",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
			},
			request: httptest.NewRequest("GET", "/api/posts", nil),
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden: Access denied",
		},
		{
			desc: "denies request when role not found",
			config: &Config{
				RoleHeader: "X-User-Role",
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermission: "admin:*"},
				},
			},
			request: httptest.NewRequest("GET", "/api", nil),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Missing or invalid role",
		},
		{
			desc: "allows request with valid role and permission",
			config: &Config{
				RoleHeader: "X-User-Role",
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermission: "admin:*"},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api", nil)
				req.Header.Set("X-User-Role", "admin")
				return req
			}(),
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			desc: "denies request with invalid permission",
			config: &Config{
				RoleHeader: "X-User-Role",
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermission: "users:write"},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api", nil)
				req.Header.Set("X-User-Role", "viewer")
				return req
			}(),
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden: Access denied",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.config != nil {
				err := tc.config.processUnifiedConfig()
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			middlewareFunc := Middleware(tc.config)
			require.NotNil(t, middlewareFunc, "TEST[%d], Failed.\n%s", i, tc.desc)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			wrapped := middlewareFunc(handler)

			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, tc.request)

			assert.Equal(t, tc.expectedStatus, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, w.Body.String(), tc.expectedBody, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
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
				req := httptest.NewRequest("GET", "/api", nil)
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
				req := httptest.NewRequest("GET", "/api", nil)
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
			request: httptest.NewRequest("GET", "/api", nil),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc: "returns error when header configured but not present",
			config: &Config{
				RoleHeader: "X-User-Role",
			},
			request: httptest.NewRequest("GET", "/api", nil),
			expectedRole: "",
			expectError:  true,
		},
		{
			desc: "returns error when no role extraction configured",
			config: &Config{},
			request: httptest.NewRequest("GET", "/api", nil),
			expectedRole: "",
			expectError:  true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			role, err := extractRole(tc.request, tc.config)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, "", role, "TEST[%d], Failed.\n%s", i, tc.desc)
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
			expectError: false,
		},
		{
			desc:      "extracts role from array claim",
			claimPath: "roles[0]",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user"},
			},
			expectedRole: "admin",
			expectError: false,
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
			expectError: false,
		},
		{
			desc:      "returns error when claims not in context",
			claimPath: "role",
			claims:    nil,
			expectedRole: "",
			expectError: true,
		},
		{
			desc:      "returns error when claim path not found",
			claimPath: "nonexistent",
			claims: jwt.MapClaims{
				"role": "admin",
			},
			expectedRole: "",
			expectError: true,
		},
		{
			desc:      "converts non-string role to string",
			claimPath: "role",
			claims: jwt.MapClaims{
				"role": 123,
			},
			expectedRole: "123",
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api", nil)

			if tc.claims != nil {
				ctx := context.WithValue(req.Context(), middleware.JWTClaim, tc.claims)
				req = req.WithContext(ctx)
			}

			role, err := extractRoleFromJWT(req, tc.claimPath)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, "", role, "TEST[%d], Failed.\n%s", i, tc.desc)
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

func TestExtractArrayClaim(t *testing.T) {
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
	}

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

func TestExtractNestedClaim(t *testing.T) {
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
	}

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
		logger   Logger
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
			req := httptest.NewRequest("GET", "/api", nil)
			logAuditEvent(tc.logger, req, "admin", "/api", tc.allowed, "test-reason")

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
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, role, route string, err error) {
					w.WriteHeader(http.StatusTeapot)
					w.Write([]byte("Custom error"))
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
			req := httptest.NewRequest("GET", "/api", nil)

			handleAuthError(w, req, tc.config, "admin", "/api", tc.err)

			assert.Equal(t, tc.expectedStatus, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, w.Body.String(), tc.expectedBody, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

