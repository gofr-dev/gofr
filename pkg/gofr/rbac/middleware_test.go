package rbac

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gofrConfig "gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

var (
	errNoRole                = errors.New("no role")
	errUserIDNotFound        = errors.New("user ID not found")
	errContainerNotAvailable = errors.New("container not available")
)

// mock role extractor function for testing.
func mockRoleExtractor(r *http.Request, _ ...any) (string, error) {
	role := r.Header.Get("Role")
	if role == "" {
		return "", errNoRole
	}

	return role, nil
}

// mock role extractor that uses container (for database-based testing).
func mockDBRoleExtractor(r *http.Request, args ...any) (string, error) {
	if len(args) == 0 {
		return "", errContainerNotAvailable
	}

	cntr, ok := args[0].(*container.Container)
	if !ok || cntr == nil {
		return "", errContainerNotAvailable
	}

	// Simulate database query
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		return "", errUserIDNotFound
	}

	// In real scenario, would query database: cntr.SQL.QueryRowContext(...)
	// For testing, return based on userID
	switch userID {
	case "1":
		return "admin", nil
	case "2":
		return "editor", nil
	default:
		return "viewer", nil
	}
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
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

			req := httptest.NewRequest(http.MethodGet, tc.requestPath, http.NoBody)
			if tc.userID != "" {
				req.Header.Set("X-User-Id", tc.userID)
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
		Logger:            logging.NewMockLogger(logging.INFO),
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
func assertErrorExpectation(t *testing.T, err error, wantErr error) {
	t.Helper()
	if wantErr != nil {
		require.Error(t, err)
		require.ErrorIs(t, err, wantErr)
		return
	}
	require.NoError(t, err)
}

// assertHandlerCallExpectation asserts handler call expectations without nested if-else.
func assertHandlerCallExpectation(t *testing.T, called bool, wantCalled bool, resp any) {
	t.Helper()
	if wantCalled {
		assert.True(t, called)
		assert.Equal(t, "success", resp)
		return
	}
	assert.False(t, called)
	assert.Nil(t, resp)
}
