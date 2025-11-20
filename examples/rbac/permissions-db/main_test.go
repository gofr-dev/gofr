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

func TestIntegration_PermissionsDB(t *testing.T) {
	// This test requires a database connection
	// For now, we'll skip if database is not available
	configs := testutil.NewServerConfigs(t)

	// Check if database is available
	// In a real scenario, you would set up test database here
	// For now, we'll create a simple test that demonstrates the pattern

	// Note: This test requires actual database setup
	// You would need to:
	// 1. Set up test database
	// 2. Create users table
	// 3. Insert test users
	// 4. Run the application
	// 5. Test endpoints

	t.Skip("Skipping database test - requires database setup")

	go main()
	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name           string
		method         string
		path           string
		userID         string
		expectedStatus int
		expectedBody   map[string]string
		checkBody      bool
	}{
		{
			name:           "GET /api/users with admin user ID - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "user1", // admin user
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users with admin user ID - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			userID:         "user1", // admin user
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "DELETE /api/users with admin user ID - should succeed",
			method:         http.MethodDelete,
			path:           "/api/users",
			userID:         "user1", // admin user
			expectedStatus: http.StatusNoContent,
			checkBody:      false,
		},
		{
			name:           "POST /api/users with editor user ID - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			userID:         "user2", // editor user
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "DELETE /api/users with editor user ID - should fail",
			method:         http.MethodDelete,
			path:           "/api/users",
			userID:         "user2", // editor user
			expectedStatus: http.StatusInternalServerError,
			checkBody:      false,
		},
		{
			name:           "GET /api/users without user ID - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "",
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

			if tc.userID != "" {
				req.Header.Set("X-User-ID", tc.userID)
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

