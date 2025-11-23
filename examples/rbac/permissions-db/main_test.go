package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/rbac"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

// setupTestDatabase creates the users table and inserts test data
func setupTestDatabase(t *testing.T, cntr *container.Container) {
	t.Helper()

	if cntr == nil || cntr.SQL == nil {
		t.Skip("Database not available - skipping database setup")
		return
	}

	// Create users table
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			email VARCHAR(255),
			role VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	_, err := cntr.SQL.Exec(createTableQuery)
	require.NoError(t, err, "Failed to create users table")

	// Clear existing data
	_, err = cntr.SQL.Exec("DELETE FROM users")
	require.NoError(t, err, "Failed to clear users table")

	// Insert test users
	insertQuery := "INSERT INTO users (id, email, role) VALUES (?, ?, ?)"
	testUsers := []struct {
		id    string
		email string
		role  string
	}{
		{"1", "admin@example.com", "admin"},
		{"2", "editor@example.com", "editor"},
		{"3", "viewer@example.com", "viewer"},
		{"4", "author@example.com", "author"},
	}

	for _, user := range testUsers {
		_, err = cntr.SQL.Exec(insertQuery, user.id, user.email, user.role)
		require.NoError(t, err, "Failed to insert test user: %s", user.id)
	}
}

// cleanupTestDatabase removes test data
func cleanupTestDatabase(t *testing.T, cntr *container.Container) {
	t.Helper()

	if cntr == nil || cntr.SQL == nil {
		return
	}

	_, err := cntr.SQL.Exec("DELETE FROM users")
	if err != nil {
		t.Logf("Warning: Failed to cleanup test database: %v", err)
	}
}

func TestIntegration_PermissionsDB_WithDatabase(t *testing.T) {
	// Check if database is available (set in GitHub workflow)
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// If DB_HOST is not set, try default GitHub workflow values
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "2001" // Default port from GitHub workflow
	}
	if dbUser == "" {
		dbUser = "root"
	}
	if dbPassword == "" {
		dbPassword = "password" // Default from GitHub workflow
	}
	if dbName == "" {
		dbName = "test" // Default from GitHub workflow
	}

	// Set database environment variables
	t.Setenv("DB_HOST", dbHost)
	t.Setenv("DB_PORT", dbPort)
	t.Setenv("DB_USER", dbUser)
	t.Setenv("DB_PASSWORD", dbPassword)
	t.Setenv("DB_NAME", dbName)
	t.Setenv("DB_DIALECT", "mysql")

	configs := testutil.NewServerConfigs(t)

	// Set up database schema using direct connection
	// We'll create a temporary container just for setup
	setupCntr := container.NewContainer(config.NewMockConfig(map[string]string{
		"DB_HOST":     dbHost,
		"DB_PORT":     dbPort,
		"DB_USER":     dbUser,
		"DB_PASSWORD": dbPassword,
		"DB_NAME":     dbName,
		"DB_DIALECT":  "mysql",
	}))

	// Wait for database connection
	time.Sleep(2 * time.Second)

	// Check if database is actually available
	if setupCntr.SQL == nil {
		t.Skip("Database not available - skipping integration test")
		return
	}

	// Test database connection
	health := setupCntr.SQL.HealthCheck()
	if health != nil && health.Status != "UP" {
		t.Skipf("Database health check failed: %v - skipping integration test", health)
		return
	}

	// Set up test database
	setupTestDatabase(t, setupCntr)
	defer cleanupTestDatabase(t, setupCntr)
	defer setupCntr.Close()

	// Start the application in a goroutine
	app := gofr.New()

	// Re-configure RBAC with the same config (app will use database from env vars)
	rbacConfig, err := rbac.LoadPermissions("configs/rbac.json")
	if err != nil {
		t.Skipf("Failed to load RBAC config: %v - skipping test", err)
		return
	}

	rbacConfig.PermissionConfig = &rbac.PermissionConfig{
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

	// CRITICAL: Set RequiresContainer = true for database-based role extraction
	// This enables container access in RoleExtractorFunc
	rbacConfig.RequiresContainer = true

	// Use database-based role extraction
	// Container is passed as args[0] when RequiresContainer = true
	rbacConfig.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
		userID := req.Header.Get("X-User-ID")
		if userID == "" {
			return "", fmt.Errorf("user ID not found in request")
		}

		// Get container from args (only available when RequiresContainer = true)
		if len(args) > 0 {
			if cntr, ok := args[0].(*container.Container); ok && cntr != nil && cntr.SQL != nil {
				var role string
				err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
				if err != nil {
					if err == sql.ErrNoRows {
						return "", fmt.Errorf("user not found")
					}
					return "", err
				}
				return role, nil
			}
		}
		return "", fmt.Errorf("database not available")
	}

	app.EnableRBACWithPermissions(rbacConfig, rbacConfig.RoleExtractorFunc)
	app.GET("/api/users", func(ctx *gofr.Context) (interface{}, error) {
		return map[string]string{"message": "Users list"}, nil
	})
	app.POST("/api/users", func(ctx *gofr.Context) (interface{}, error) {
		return map[string]string{"message": "User created"}, nil
	})
	app.DELETE("/api/users", gofr.RequirePermission("users:delete", rbacConfig.PermissionConfig, func(ctx *gofr.Context) (interface{}, error) {
		return map[string]string{"message": "User deleted"}, nil
	}))
	app.GET("/api/posts", func(ctx *gofr.Context) (interface{}, error) {
		return map[string]string{"message": "Posts list"}, nil
	})
	app.POST("/api/posts", func(ctx *gofr.Context) (interface{}, error) {
		return map[string]string{"message": "Post created"}, nil
	})

	go func() {
		app.Run()
	}()
	time.Sleep(500 * time.Millisecond) // Give server time to start

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name           string
		method         string
		path           string
		userID         string
		body           []byte
		expectedStatus int
		expectedBody   map[string]string
		checkBody      bool
	}{
		{
			name:           "GET /api/users with admin user ID (1) - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "1", // admin user
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users with editor user ID (2) - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "2", // editor user
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "GET /api/users with viewer user ID (3) - should succeed",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "3", // viewer user
			expectedStatus: http.StatusOK,
			expectedBody:   map[string]string{"message": "Users list"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users with admin user ID (1) - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			userID:         "1", // admin user
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users with editor user ID (2) - should succeed",
			method:         http.MethodPost,
			path:           "/api/users",
			userID:         "2", // editor user
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"message": "User created"},
			checkBody:      true,
		},
		{
			name:           "POST /api/users with viewer user ID (3) - should fail (no users:write)",
			method:         http.MethodPost,
			path:           "/api/users",
			userID:         "3", // viewer user
			body:           []byte(`{"name":"test"}`),
			expectedStatus: http.StatusForbidden,
			checkBody:      false,
		},
		{
			name:           "DELETE /api/users with admin user ID (1) - should succeed",
			method:         http.MethodDelete,
			path:           "/api/users",
			userID:         "1", // admin user
			expectedStatus: http.StatusNoContent,
			checkBody:      false,
		},
		{
			name:           "DELETE /api/users with editor user ID (2) - should fail (no users:delete)",
			method:         http.MethodDelete,
			path:           "/api/users",
			userID:         "2", // editor user
			expectedStatus: http.StatusInternalServerError, // RequirePermission returns error which becomes 500
			checkBody:      false,
		},
		{
			name:           "GET /api/users without user ID - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "",
			expectedStatus: http.StatusUnauthorized, // Role extractor returns error
			checkBody:      false,
		},
		{
			name:           "GET /api/users with non-existent user ID - should fail",
			method:         http.MethodGet,
			path:           "/api/users",
			userID:         "999", // non-existent user
			expectedStatus: http.StatusUnauthorized, // User not found in database
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

// TestDatabaseRoleExtraction tests the role extraction directly from database
func TestDatabaseRoleExtraction(t *testing.T) {
	// Check if database is available
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "2001"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "root"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "password"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "test"
	}

	t.Setenv("DB_HOST", dbHost)
	t.Setenv("DB_PORT", dbPort)
	t.Setenv("DB_USER", dbUser)
	t.Setenv("DB_PASSWORD", dbPassword)
	t.Setenv("DB_NAME", dbName)
	t.Setenv("DB_DIALECT", "mysql")

	// Create container for database access
	cntr := container.NewContainer(config.NewMockConfig(map[string]string{
		"DB_HOST":     dbHost,
		"DB_PORT":     dbPort,
		"DB_USER":     dbUser,
		"DB_PASSWORD": dbPassword,
		"DB_NAME":     dbName,
		"DB_DIALECT":  "mysql",
	}))

	// Wait for database connection
	time.Sleep(2 * time.Second)

	if cntr.SQL == nil {
		t.Skip("Database not available - skipping database role extraction test")
		return
	}

	// Test database connection
	health := cntr.SQL.HealthCheck()
	if health != nil && health.Status != "UP" {
		t.Skipf("Database health check failed: %v - skipping test", health)
		return
	}

	// Set up test database
	setupTestDatabase(t, cntr)
	defer cleanupTestDatabase(t, cntr)
	defer cntr.Close()

	// Test role extraction using container.SQL
	tests := []struct {
		name       string
		userID     string
		expectRole string
		expectErr  bool
	}{
		{
			name:       "Extract admin role for user ID 1",
			userID:     "1",
			expectRole: "admin",
			expectErr:  false,
		},
		{
			name:       "Extract editor role for user ID 2",
			userID:     "2",
			expectRole: "editor",
			expectErr:  false,
		},
		{
			name:       "Extract viewer role for user ID 3",
			userID:     "3",
			expectRole: "viewer",
			expectErr:  false,
		},
		{
			name:       "Non-existent user should return error",
			userID:     "999",
			expectRole: "",
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var role string
			err := cntr.SQL.QueryRow(
				"SELECT role FROM users WHERE id = ?",
				tc.userID,
			).Scan(&role)

			if tc.expectErr {
				assert.Error(t, err, "Expected error for non-existent user")
				assert.Equal(t, sql.ErrNoRows, err, "Expected sql.ErrNoRows")
			} else {
				require.NoError(t, err, "Failed to query role from database")
				assert.Equal(t, tc.expectRole, role, "Role mismatch for user ID %s", tc.userID)
			}
		})
	}
}
