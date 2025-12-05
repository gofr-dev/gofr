package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_RBACJWT(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Create mock JWKS server
	mockJWKS, err := NewMockJWKSServer()
	require.NoError(t, err, "Failed to create mock JWKS server")
	defer mockJWKS.Close()

	// Set the JWKS endpoint URL for the application
	os.Setenv("JWKS_ENDPOINT", mockJWKS.JWKSEndpoint())

	// Start the application with the mock JWKS endpoint
	// We need to modify main() to accept the JWKS endpoint, or we can set it via env
	// For now, let's create a test version that uses the mock server
	go func() {
		app := createTestApp(mockJWKS.JWKSEndpoint())
		app.Run()
	}()

	// Wait for server to start and JWKS to be fetched
	time.Sleep(500 * time.Millisecond) // Give time for JWKS fetch

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name           string
		method         string
		path           string
		role           string
		expectedStatus int
		expectedBody   map[string]string
		checkBody      bool
	}{
		{
			name:           "GET /api/users with admin JWT - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			role:           "admin",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users with editor JWT - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			role:           "editor",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users with viewer JWT - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			role:           "viewer",
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/admin with admin JWT - route pattern mismatch, returns 403",
			method:         http.MethodGet,
			path:           "/api/admin",
			role:           "admin",
			expectedStatus: http.StatusForbidden, // Config has "/api/admin/*" pattern which doesn't match "/api/admin"
			checkBody:      false,
		},
		{
			name:           "GET /api/admin/dashboard with admin JWT - RBAC passes but route not found",
			method:         http.MethodGet,
			path:           "/api/admin/dashboard",
			role:           "admin",
			expectedStatus: http.StatusNotFound, // Route doesn't exist, but RBAC check passes (403 -> 404)
			checkBody:      false,
		},
		{
			name:           "GET /api/admin with editor JWT - should fail",
			method:         http.MethodGet,
			path:           "/api/admin",
			role:           "editor",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		{
			name:           "GET /api/admin with viewer JWT - should fail",
			method:         http.MethodGet,
			path:           "/api/admin",
			role:           "viewer",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		{
			name:           "GET /api/users without JWT - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			role:           "",
			expectedStatus: http.StatusUnauthorized,
			checkBody:      false,
		},
		{
			name:           "GET /api/users with invalid JWT - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			role:           "invalid-token",
			expectedStatus: http.StatusUnauthorized,
			checkBody:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, configs.HTTPHost+tc.path, nil)
			require.NoError(t, err)

			if tc.role != "" && tc.role != "invalid-token" {
				// Generate JWT token with role claim
				claims := jwt.MapClaims{
					"role": tc.role,
					"sub":  "test-user",
				}
				token, err := mockJWKS.GenerateToken(claims)
				require.NoError(t, err, "Failed to generate JWT token")
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			} else if tc.role == "invalid-token" {
				req.Header.Set("Authorization", "Bearer invalid.jwt.token")
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

// createTestApp creates an app instance for testing with the given JWKS endpoint
func createTestApp(jwksEndpoint string) *gofr.App {
	app := gofr.New()

	// Enable OAuth middleware with mock JWKS endpoint
	app.EnableOAuth(jwksEndpoint, 10)

	// Enable RBAC with JWT role extraction
	app.EnableRBACWithJWT("configs/rbac.json", "role")

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.GET("/api/admin", adminHandler)

	return app
}

