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
	"gofr.dev/pkg/gofr/rbac"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_PermissionsJWT(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Create mock JWKS server
	mockJWKS, err := NewMockJWKSServer()
	require.NoError(t, err, "Failed to create mock JWKS server")
	defer mockJWKS.Close()

	// Start the application with the mock JWKS endpoint
	go func() {
		app := createTestApp(mockJWKS.JWKSEndpoint())
		app.Run()
	}()

	// Wait for server to start and JWKS to be fetched
	time.Sleep(500 * time.Millisecond)

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
			name:           "POST /api/users with admin JWT - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			role:           "admin",
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "DELETE /api/users with admin JWT - should succeed",
			method:         http.MethodDelete,
			path:           "/api/users",
			role:           "admin",
			expectedStatus: http.StatusNoContent,
			checkBody:      false,
		},
		{
			name:           "POST /api/users with viewer JWT - should fail (no users:write)",
			method:         http.MethodPost,
			path:           "/api/users",
			role:           "viewer",
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		{
			name:           "DELETE /api/users with editor JWT - should fail (no users:delete)",
			method:         http.MethodDelete,
			path:           "/api/users",
			role:           "editor",
			expectedStatus: http.StatusInternalServerError,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, configs.HTTPHost+tc.path, nil)
			require.NoError(t, err)

			if tc.role != "" {
				// Generate JWT token with role claim
				claims := jwt.MapClaims{
					"role": tc.role,
					"sub":  "test-user",
				}
				token, err := mockJWKS.GenerateToken(claims)
				require.NoError(t, err, "Failed to generate JWT token")
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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

	// Load RBAC configuration
	config, err := rbac.LoadPermissions("configs/rbac.json")
	if err != nil {
		app.Logger().Error("Failed to load RBAC config: ", err)
		return app
	}

	// Configure permission-based access control
	config.PermissionConfig = &rbac.PermissionConfig{
		Permissions: map[string][]string{
			"users:read":   {"admin", "editor", "viewer"},
			"users:write":  {"admin", "editor"},
			"users:delete": {"admin"},
			"posts:read":   {"admin", "author", "viewer"},
			"posts:write":  {"admin", "author"},
		},
		RoutePermissionMap: map[string]string{
			"GET /api/users":    "users:read",
			"POST /api/users":   "users:write",
			"DELETE /api/users": "users:delete",
			"GET /api/posts":    "posts:read",
			"POST /api/posts":   "posts:write",
		},
	}

	// Create JWT role extractor
	jwtExtractor := rbac.NewJWTRoleExtractor("role")
	config.RoleExtractorFunc = jwtExtractor.ExtractRole

	// Enable RBAC with permissions
	app.EnableRBACWithPermissions(config, jwtExtractor.ExtractRole)

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.POST("/api/users", createUser)
	app.DELETE("/api/users", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
	app.GET("/api/posts", getAllPosts)
	app.POST("/api/posts", createPost)

	return app
}

