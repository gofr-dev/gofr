package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errNoRole           = errors.New("no role")
	errExtractionFailed = errors.New("extraction failed")
)

// mock role extractor function for testing.
func mockRoleExtractor(r *http.Request, _ ...any) (string, error) {
	role := r.Header.Get("Role")
	if role == "" {
		return "", errNoRole
	}

	return role, nil
}

func TestMiddleware_Authorization(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/allowed": {"admin"},
		},
		OverRides:         map[string]bool{},
		RoleExtractorFunc: mockRoleExtractor,
	}

	// next handler to confirm request passed through middleware
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	// Test without container (header-based RBAC doesn't need container)
	middleware := Middleware(config)

	// test cases
	tests := []struct {
		name         string
		roleHeader   string
		requestPath  string
		wantStatus   int
		wantNextCall bool
	}{
		{
			name:         "No role header",
			roleHeader:   "",
			requestPath:  "/allowed",
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
		{
			name:         "Unauthorized role",
			roleHeader:   "user",
			requestPath:  "/allowed",
			wantStatus:   http.StatusForbidden,
			wantNextCall: false,
		},
		{
			name:         "Authorized role",
			roleHeader:   "admin",
			requestPath:  "/allowed",
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled = false

			req := httptest.NewRequest(http.MethodGet, tc.requestPath, http.NoBody)

			if tc.roleHeader != "" {
				req.Header.Set("Role", tc.roleHeader)
			}

			w := httptest.NewRecorder()

			handlerToTest := middleware(nextHandler)
			handlerToTest.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantNextCall, nextCalled)
		})
	}
}

func TestMiddleware_WithOverride(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/public": {"admin"},
		},
		OverRides: map[string]bool{
			"/public": true,
		},
		RoleExtractorFunc: mockRoleExtractor,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/public", http.NoBody)
	// No role header - should still pass due to override
	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, nextCalled)
}

func TestMiddleware_WithDefaultRole(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/allowed": {"viewer"},
		},
		DefaultRole: "viewer",
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "", errNoRole
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", http.NoBody)
	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, nextCalled)
}

func TestMiddleware_WithPermissionCheck(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
		EnablePermissions: true,
		PermissionConfig: &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin", "editor"},
			},
			RoutePermissionMap: map[string]string{
				"GET /api/users": "users:read",
			},
		},
		RoleExtractorFunc: mockRoleExtractor,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)

	tests := []struct {
		name         string
		role         string
		wantStatus   int
		wantNextCall bool
	}{
		{
			name:         "Has permission",
			role:         "editor",
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
		{
			name:         "No permission",
			role:         "viewer",
			wantStatus:   http.StatusForbidden,
			wantNextCall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)

			req.Header.Set("Role", tc.role)

			w := httptest.NewRecorder()

			handlerToTest := middleware(nextHandler)
			handlerToTest.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantNextCall, nextCalled)
		})
	}
}

func TestMiddleware_WithHierarchy(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"editor"},
		},
		RoleHierarchy: map[string][]string{
			"admin": {"editor", "viewer"},
		},
		RoleExtractorFunc: mockRoleExtractor,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.Header.Set("Role", "admin") // Admin should have editor permissions

	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, nextCalled)
}

func TestMiddleware_WithCustomErrorHandler(t *testing.T) {
	errorHandlerCalled := false
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/allowed": {"admin"},
		},
		RoleExtractorFunc: mockRoleExtractor,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _, _ string, _ error) {
			errorHandlerCalled = true
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("Custom error"))
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", http.NoBody)
	req.Header.Set("Role", "user") // Unauthorized

	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Custom error")
}

func TestMiddleware_WithAuditLogging(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/allowed": {"admin"},
		},
		RoleExtractorFunc: mockRoleExtractor,
		Logger:            &mockLogger{},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	// Audit logging is automatically performed when Logger is set
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", http.NoBody)

	req.Header.Set("Role", "admin")

	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	// Audit logging happens automatically - no need to verify it was called
	// The middleware will log using GoFr's logger when Logger is set
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Handler(t *testing.T) {
	allowedRole := "admin"
	called := false
	handlerFunc := func(_ any) (any, error) {
		called = true
		return "success", nil
	}

	wrappedHandler := RequireRole(allowedRole, handlerFunc)

	tests := []struct {
		name        string
		contextRole string
		wantErr     error
		wantCalled  bool
	}{
		{
			name:        "Role allowed",
			contextRole: "admin",
			wantErr:     nil,
			wantCalled:  true,
		},
		{
			name:        "Role denied",
			contextRole: "user",
			wantErr:     ErrAccessDenied,
			wantCalled:  false,
		},
		{
			name:        "No role in context",
			contextRole: "",
			wantErr:     ErrAccessDenied,
			wantCalled:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			ctx := &mockContextValueGetter{
				value: func(key any) any {
					if key == userRole {
						return tc.contextRole
					}
					return nil
				},
			}
			resp, err := wrappedHandler(ctx)

			assertErrorExpectation(t, err, tc.wantErr)
			assertHandlerCallExpectation(t, called, tc.wantCalled, resp)
		})
	}
}

func TestRequireAnyRole(t *testing.T) {
	allowedRoles := []string{"admin", "editor"}
	called := false
	handlerFunc := func(_ any) (any, error) {
		called = true
		return "success", nil
	}

	wrappedHandler := RequireAnyRole(allowedRoles, handlerFunc)

	tests := []struct {
		name        string
		contextRole string
		wantErr     error
		wantCalled  bool
	}{
		{
			name:        "First role allowed",
			contextRole: "admin",
			wantErr:     nil,
			wantCalled:  true,
		},
		{
			name:        "Second role allowed",
			contextRole: "editor",
			wantErr:     nil,
			wantCalled:  true,
		},
		{
			name:        "Role denied",
			contextRole: "viewer",
			wantErr:     ErrAccessDenied,
			wantCalled:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			ctx := &mockContextValueGetter{
				value: func(key any) any {
					if key == userRole {
						return tc.contextRole
					}
					return nil
				},
			}
			resp, err := wrappedHandler(ctx)

			assertErrorExpectation(t, err, tc.wantErr)
			assertHandlerCallExpectation(t, called, tc.wantCalled, resp)
		})
	}
}

// assertErrorExpectation asserts error expectations without nested if-else.
func assertErrorExpectation(t *testing.T, err, wantErr error) {
	t.Helper()

	if wantErr != nil {
		require.Error(t, err)
		require.ErrorIs(t, err, wantErr)

		return
	}

	require.NoError(t, err)
}

// assertHandlerCallExpectation asserts handler call expectations without nested if-else.
func assertHandlerCallExpectation(t *testing.T, called, wantCalled bool, resp any) {
	t.Helper()

	if wantCalled {
		assert.True(t, called)
		assert.Equal(t, "success", resp)

		return
	}

	assert.False(t, called)
	assert.Nil(t, resp)
}

func TestExtractRole_EdgeCases(t *testing.T) {
	t.Run("NilRoleExtractorWithDefaultRole", func(t *testing.T) {
		config := &Config{
			DefaultRole: "guest",
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		role, err := extractRole(req, config)
		require.NoError(t, err)
		assert.Equal(t, "guest", role)
	})

	t.Run("RoleExtractorErrorWithDefaultRole", func(t *testing.T) {
		config := &Config{
			DefaultRole: "guest",
			RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
				return "", errExtractionFailed
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		role, err := extractRole(req, config)
		require.NoError(t, err)
		assert.Equal(t, "guest", role)
	})

	t.Run("RoleExtractorErrorNoDefaultRole", func(t *testing.T) {
		config := &Config{
			RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
				return "", errExtractionFailed
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		role, err := extractRole(req, config)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrRoleNotFound)
		assert.Empty(t, role)
	})

	t.Run("RoleExtractorEmptyStringWithDefaultRole", func(t *testing.T) {
		config := &Config{
			DefaultRole: "guest",
			RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
				return "", nil // Empty string, no error
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		role, err := extractRole(req, config)
		require.NoError(t, err)
		assert.Equal(t, "guest", role)
	})

	t.Run("RoleExtractorEmptyStringNoDefaultRole", func(t *testing.T) {
		config := &Config{
			RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
				return "", nil // Empty string, no error
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		role, err := extractRole(req, config)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrRoleNotFound)
		assert.Empty(t, role)
	})
}


func TestCheckAuthorization_EdgeCases(t *testing.T) {
	t.Run("PermissionBasedAccess", func(t *testing.T) {
		config := &Config{
			EnablePermissions: true,
			PermissionConfig: &PermissionConfig{
				Permissions: map[string][]string{
					"users:read": {"admin"},
				},
				RoutePermissionMap: map[string]string{
					"GET /api/users": "users:read",
				},
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
		reqCtx := context.WithValue(req.Context(), userRole, "admin")
		req = req.WithContext(reqCtx)

		authorized, reason := checkAuthorization(req, "admin", "/api/users", config)
		assert.True(t, authorized)
		assert.Equal(t, authReasonPermissionBased, reason)
	})

	t.Run("RoleBasedWithHierarchy", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/api/users": {"editor"},
			},
			RoleHierarchy: map[string][]string{
				"admin": {"editor"},
			},
		}

		authorized, reason := checkAuthorization(nil, "admin", "/api/users", config)
		assert.True(t, authorized)
		assert.Equal(t, authReasonRoleBasedHierarchy, reason)
	})

	t.Run("RoleBasedNoHierarchy", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/api/users": {"admin"},
			},
		}

		authorized, reason := checkAuthorization(nil, "admin", "/api/users", config)
		assert.True(t, authorized)
		assert.Equal(t, authReasonRoleBased, reason)
	})

	t.Run("NotAuthorized", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/api/users": {"admin"},
			},
		}

		authorized, reason := checkAuthorization(nil, "viewer", "/api/users", config)
		assert.False(t, authorized)
		assert.Empty(t, reason)
	})
}

func TestMiddleware_ExtractRoleError(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/test": {"admin"},
		},
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "", errExtractionFailed
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_ExtractRoleWithDefaultRole(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/test": {"guest"},
		},
		DefaultRole: "guest",
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "", errExtractionFailed
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	handlerToTest := middleware(nextHandler)
	handlerToTest.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAuthError(t *testing.T) {
	t.Run("WithRoleNotFoundError", func(t *testing.T) {
		config := &Config{
			Logger: &mockLogger{},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()

		handleAuthError(w, req, config, "", "/test", ErrRoleNotFound)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Unauthorized")
	})

	t.Run("WithAccessDeniedError", func(t *testing.T) {
		config := &Config{
			Logger: &mockLogger{},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()

		handleAuthError(w, req, config, "viewer", "/test", ErrAccessDenied)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Forbidden")
	})

	t.Run("WithCustomErrorHandler", func(t *testing.T) {
		errorHandlerCalled := false
		config := &Config{
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _, _ string, _ error) {
				errorHandlerCalled = true
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("Custom error"))
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()

		handleAuthError(w, req, config, "viewer", "/test", ErrAccessDenied)

		assert.True(t, errorHandlerCalled)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Custom error")
	})

	t.Run("WithNilLogger", func(t *testing.T) {
		config := &Config{}

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()

		handleAuthError(w, req, config, "viewer", "/test", ErrAccessDenied)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestCheckAuthorization_PermissionBasedDisabled(t *testing.T) {
	config := &Config{
		EnablePermissions: false,
		PermissionConfig: &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		},
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
	}

	authorized, reason := checkAuthorization(nil, "admin", "/api/users", config)
	assert.True(t, authorized)
	assert.Equal(t, authReasonRoleBased, reason) // Should use role-based, not permission-based
}

func TestCheckAuthorization_PermissionBasedNoConfig(t *testing.T) {
	config := &Config{
		EnablePermissions: true,
		PermissionConfig:  nil, // No permission config
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
	}

	authorized, reason := checkAuthorization(nil, "admin", "/api/users", config)
	assert.True(t, authorized)
	assert.Equal(t, authReasonRoleBased, reason) // Should fallback to role-based
}

func TestCheckAuthorization_EmptyHierarchy(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
		RoleHierarchy: map[string][]string{}, // Empty hierarchy
	}

	authorized, reason := checkAuthorization(nil, "admin", "/api/users", config)
	assert.True(t, authorized)
	assert.Equal(t, authReasonRoleBased, reason) // Should use role-based when hierarchy is empty
}
