package rbac

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	gofrConfig "gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

// mock role extractor function for testing
func mockRoleExtractor(r *http.Request, args ...any) (string, error) {
	role := r.Header.Get("Role")
	if role == "" {
		return "", errors.New("no role")
	}
	return role, nil
}

// mock role extractor that uses container (for database-based testing)
func mockDBRoleExtractor(r *http.Request, args ...any) (string, error) {
	// Check if container is provided
	if len(args) > 0 {
		if cntr, ok := args[0].(*container.Container); ok && cntr != nil {
			// Simulate database query
			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				return "", errors.New("user ID not found")
			}
			// In real scenario, would query database: cntr.SQL.QueryRowContext(...)
			// For testing, return based on userID
			if userID == "1" {
				return "admin", nil
			}
			if userID == "2" {
				return "editor", nil
			}
			return "viewer", nil
		}
	}
	return "", errors.New("container not available")
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
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			req := httptest.NewRequest(http.MethodGet, tc.requestPath, nil)
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

func TestMiddleware_WithContainer(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/allowed": {"admin"},
		},
		OverRides:         map[string]bool{},
		RoleExtractorFunc: mockDBRoleExtractor,
		RequiresContainer: true, // Enable container access for database-based extraction
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Test with container (database-based RBAC needs container)
	mockContainer := container.NewContainer(gofrConfig.NewMockConfig(nil))
	middleware := Middleware(config, mockContainer)

	tests := []struct {
		name         string
		userID       string
		requestPath  string
		wantStatus   int
		wantNextCall bool
	}{
		{
			name:         "Admin user (userID=1)",
			userID:       "1",
			requestPath:  "/allowed",
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
		{
			name:         "Editor user (userID=2)",
			userID:       "2",
			requestPath:  "/allowed",
			wantStatus:   http.StatusForbidden,
			wantNextCall: false,
		},
		{
			name:         "No user ID",
			userID:       "",
			requestPath:  "/allowed",
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, tc.requestPath, nil)
			if tc.userID != "" {
				req.Header.Set("X-User-ID", tc.userID)
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
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
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
		DefaultRole:       "viewer",
		RoleExtractorFunc: func(r *http.Request, args ...any) (string, error) {
			return "", errors.New("no role")
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
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
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
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
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
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
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, role, route string, err error) {
			errorHandlerCalled = true
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Custom error"))
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
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
		Logger:            logging.NewMockLogger(logging.INFO),
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without container
	// Audit logging is automatically performed when Logger is set
	middleware := Middleware(config)
	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
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
	handlerFunc := func(ctx interface{}) (any, error) {
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
				value: func(key interface{}) interface{} {
					if key == userRole {
						return tc.contextRole
					}
					return nil
				},
			}
			resp, err := wrappedHandler(ctx)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
			if tc.wantCalled {
				assert.True(t, called)
				assert.Equal(t, "success", resp)
			} else {
				assert.False(t, called)
				assert.Nil(t, resp)
			}
		})
	}
}

func TestRequireAnyRole(t *testing.T) {
	allowedRoles := []string{"admin", "editor"}
	called := false
	handlerFunc := func(ctx interface{}) (any, error) {
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
				value: func(key interface{}) interface{} {
					if key == userRole {
						return tc.contextRole
					}
					return nil
				},
			}
			resp, err := wrappedHandler(ctx)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
			if tc.wantCalled {
				assert.True(t, called)
				assert.Equal(t, "success", resp)
			} else {
				assert.False(t, called)
				assert.Nil(t, resp)
			}
		})
	}
}

