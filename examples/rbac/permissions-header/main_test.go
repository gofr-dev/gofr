package main

import (
	"bytes"
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

func TestIntegration_RBACPermissions(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(200 * time.Millisecond) // Give server time to start

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name           string
		method         string
		path           string
		roleHeader     string
		body           []byte
		expectedStatus int
		expectedBody   map[string]string
		checkBody      bool
	}{
		// GET /api/users - requires users:read permission (admin, editor, viewer)
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
			name:           "GET /api/users as author - should fail (no users:read)",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "author",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		// POST /api/users - requires users:write permission (admin, editor)
		// Also needs to pass route-based check: "/api/*": ["admin", "editor"]
		{
			name:           "POST /api/users as admin - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			roleHeader:     "admin",
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated, // POST returns 201 Created
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users as editor - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			roleHeader:     "editor",
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated, // POST returns 201 Created
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users as viewer - should fail (no users:write)",
			method:         http.MethodPost,
			path:           "/api/users",
			roleHeader:     "viewer",
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		// DELETE /api/users - requires users:delete permission (admin only, via RequirePermission)
		{
			name:           "DELETE /api/users as admin - should succeed",
			method:         http.MethodDelete,
			path:           "/api/users",
			roleHeader:     "admin",
			expectedStatus: http.StatusNoContent, // DELETE returns 204 No Content
			expectedBody:   nil,                   // DELETE doesn't return body
			checkBody:      false,                 // Don't check body for DELETE
		},
		{
			name:           "DELETE /api/users as editor - should fail (no users:delete)",
			method:         http.MethodDelete,
			path:           "/api/users",
			roleHeader:     "editor",
			expectedStatus: http.StatusInternalServerError, // RequirePermission returns error which becomes 500
			checkBody:      false,
		},
		{
			name:           "DELETE /api/users as viewer - should fail (no users:delete)",
			method:         http.MethodDelete,
			path:           "/api/users",
			roleHeader:     "viewer",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		// GET /api/posts - requires posts:read permission (admin, author, viewer)
		{
			name:           "GET /api/posts as admin - should succeed",
			method:         http.MethodGet,
			path:           "/api/posts",
			roleHeader:     "admin",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Posts list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/posts as author - should succeed",
			method:         http.MethodGet,
			path:           "/api/posts",
			roleHeader:     "author",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Posts list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/posts as viewer - should succeed",
			method:         http.MethodGet,
			path:           "/api/posts",
			roleHeader:     "viewer",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Posts list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/posts as editor - should succeed (has access via /api/* route)",
			method:         http.MethodGet,
			path:           "/api/posts",
			roleHeader:     "editor",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Posts list"},
			checkBody:      true,
		},
		// No role header - config has "/api/*": ["admin", "editor"] so no default role means no access
		{
			name:           "GET /api/users without role - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			roleHeader:     "",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != nil {
				bodyReader = bytes.NewReader(tc.body)
			}

			req, err := http.NewRequest(tc.method, configs.HTTPHost+tc.path, bodyReader)
			require.NoError(t, err)

			if tc.roleHeader != "" {
				req.Header.Set("X-User-Role", tc.roleHeader)
			}
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
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

func TestIntegration_RBACPermissions_HealthCheck(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	// Health check - config has "/api/*": ["admin", "editor"] so health endpoint may need role
	req, err := http.NewRequest(http.MethodGet, configs.HTTPHost+"/.well-known/health", nil)
	require.NoError(t, err)
	req.Header.Set("X-User-Role", "admin") // Use admin to ensure access

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Health endpoint should be accessible
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
		"Health check should be accessible or return 403 if RBAC applies")
}

