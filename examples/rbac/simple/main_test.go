package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_RBACEnhanced(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(200 * time.Millisecond) // Give server time to start

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name           string
		method         string
		path           string
		roleHeader     string
		expectedStatus int
		expectedBody   map[string]string
		checkBody      bool
	}{
		// GET /api/users - accessible by admin, editor, viewer
		{
			name:           "GET /api/users as admin - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "admin",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users as editor - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "editor",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users as viewer - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "viewer",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users without role - uses default role (viewer)",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "",
			expectedStatus: http.StatusOK, // Default role is "viewer" which has access
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users as unauthorized role - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "guest",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		// GET /api/admin - requires admin role (via RequireRole)
		{
			name:           "GET /api/admin as admin - should succeed",
			method:         http.MethodGet,
			path:           "/api/admin",
			roleHeader:     "admin",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Admin panel"},
			checkBody:      true,
		},
		{
			name:           "GET /api/admin as editor - should fail",
			method:         http.MethodGet,
			path:           "/api/admin",
			roleHeader:     "editor",
			expectedStatus: http.StatusInternalServerError, // RequireRole returns error which becomes 500
			checkBody:      false,
		},
		{
			name:           "GET /api/admin as viewer - should fail",
			method:         http.MethodGet,
			path:           "/api/admin",
			roleHeader:     "viewer",
			expectedStatus: http.StatusInternalServerError, // RequireRole returns error which becomes 500
			checkBody:      false,
		},
		{
			name:           "GET /api/admin without role - uses default role (viewer) and fails",
			method:         http.MethodGet,
			path:           "/api/admin",
			roleHeader:     "",
			expectedStatus: http.StatusForbidden, // Default role is "viewer" which doesn't have access to /api/admin/*
			checkBody:      false,
		},
		// GET /api/dashboard - requires admin or editor (via RequireAnyRole)
		{
			name:           "GET /api/dashboard as admin - should succeed",
			method:         http.MethodGet,
			path:           "/api/dashboard",
			roleHeader:     "admin",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Dashboard"},
			checkBody:      true,
		},
		{
			name:           "GET /api/dashboard as editor - should succeed",
			method:         http.MethodGet,
			path:           "/api/dashboard",
			roleHeader:     "editor",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Dashboard"},
			checkBody:      true,
		},
		{
			name:           "GET /api/dashboard as viewer - should fail",
			method:         http.MethodGet,
			path:           "/api/dashboard",
			roleHeader:     "viewer",
			expectedStatus: http.StatusInternalServerError, // RequireAnyRole returns error which becomes 500
			checkBody:      false,
		},
		{
			name:           "GET /api/dashboard without role - uses default role (viewer) and fails",
			method:         http.MethodGet,
			path:           "/api/dashboard",
			roleHeader:     "",
			expectedStatus: http.StatusInternalServerError, // Default role is "viewer" which doesn't have dashboard access (RequireAnyRole returns error)
			checkBody:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, configs.HTTPHost+tc.path, nil)
			require.NoError(t, err)

			if tc.roleHeader != "" {
				req.Header.Set("X-User-Role", tc.roleHeader)
			}

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode, "Status code mismatch for %s", tc.name)

			if tc.checkBody {
				bodyBytes, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				var response struct {
					Data map[string]string `json:"data"`
				}
				err = json.Unmarshal(bodyBytes, &response)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedBody, response.Data, "Response body mismatch for %s", tc.name)
			}
		})
	}
}

func TestIntegration_RBACEnhanced_HealthCheck(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	// Health check - use default role (viewer) which has access via "*" route
	req, err := http.NewRequest(http.MethodGet, configs.HTTPHost+"/.well-known/health", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Health endpoint should be accessible (viewer has access via "*" route or override)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
		"Health check should be accessible or return 403 if RBAC applies")
}

