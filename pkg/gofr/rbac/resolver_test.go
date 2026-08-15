package rbac

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConfig builds a Config from endpoints and processes it, failing the test on error.
func newTestConfig(tb testing.TB, endpoints []EndpointMapping, roles []RoleDefinition) *Config {
	tb.Helper()

	config := &Config{Endpoints: endpoints, Roles: roles, RoleHeader: "X-User-Role"}
	require.NoError(tb, config.processUnifiedConfig())

	return config
}

func TestGetEndpointForRequest_WildcardMethod(t *testing.T) {
	testCases := []struct {
		desc          string
		methods       []string
		requestMethod string
		expectMatch   bool
	}{
		{"wildcard matches GET", []string{"*"}, http.MethodGet, true},
		{"wildcard matches DELETE", []string{"*"}, http.MethodDelete, true},
		{"wildcard matches custom method", []string{"*"}, "PROPFIND", true},
		{"omitted methods matches DELETE", nil, http.MethodDelete, true},
		{"empty methods matches DELETE", []string{}, http.MethodDelete, true},
		{"explicit method matches itself", []string{"GET"}, http.MethodGet, true},
		{"explicit method does not match other", []string{"GET"}, http.MethodDelete, false},
		{"lowercase declaration matches", []string{"get"}, http.MethodGet, true},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := newTestConfig(t, []EndpointMapping{
				{Path: "/admin/{path:.*}", Methods: tc.methods, RequiredPermissions: []string{"admin:read"}},
			}, nil)

			req := httptest.NewRequestWithContext(t.Context(), tc.requestMethod, "/admin/orgs/123", http.NoBody)
			endpoint, isPublic := getEndpointForRequest(req, config)

			assert.False(t, isPublic)

			if tc.expectMatch {
				require.NotNil(t, endpoint, "expected the rule to match")
				assert.Equal(t, "/admin/{path:.*}", endpoint.Path)
			} else {
				assert.Nil(t, endpoint)
			}
		})
	}
}

func TestGetEndpointForRequest_MostSpecificWins(t *testing.T) {
	broad := EndpointMapping{Path: "/admin/{path:.*}", Methods: []string{"*"}, RequiredPermissions: []string{"admin:read"}}
	narrow := EndpointMapping{Path: "/admin/orgs/{org_id}", Methods: []string{"DELETE"}, RequiredPermissions: []string{"admin:write"}}
	literal := EndpointMapping{Path: "/admin/orgs/global", Methods: []string{"DELETE"}, RequiredPermissions: []string{"admin:super"}}

	testCases := []struct {
		desc      string
		endpoints []EndpointMapping
		path      string
		expected  string
	}{
		{"narrow wins over broad", []EndpointMapping{broad, narrow}, "/admin/orgs/123", "/admin/orgs/{org_id}"},
		{"declaration order is irrelevant", []EndpointMapping{narrow, broad}, "/admin/orgs/123", "/admin/orgs/{org_id}"},
		{"literal wins over param", []EndpointMapping{broad, narrow, literal}, "/admin/orgs/global", "/admin/orgs/global"},
		{"broad wins when it is the only match", []EndpointMapping{broad, narrow}, "/admin/settings", "/admin/{path:.*}"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := newTestConfig(t, tc.endpoints, nil)

			// Repeat to catch map-iteration nondeterminism: Go randomizes range order per range.
			for range 100 {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, tc.path, http.NoBody)
				endpoint, _ := getEndpointForRequest(req, config)

				require.NotNil(t, endpoint)
				require.Equal(t, tc.expected, endpoint.Path)
			}
		})
	}
}

func TestGetEndpointForRequest_ExplicitMethodBeatsWildcard(t *testing.T) {
	config := newTestConfig(t, []EndpointMapping{
		{Path: "/admin/reports", Methods: []string{"*"}, RequiredPermissions: []string{"admin:read"}},
		{Path: "/admin/reports", Methods: []string{"DELETE"}, RequiredPermissions: []string{"admin:write"}},
	}, nil)

	for range 100 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/reports", http.NoBody)
		endpoint, _ := getEndpointForRequest(req, config)

		require.NotNil(t, endpoint)
		require.Equal(t, []string{"admin:write"}, endpoint.RequiredPermissions)
	}
}

func TestConfig_GetEndpointPermission_WildcardMethod(t *testing.T) {
	testCases := []struct {
		desc           string
		method         string
		path           string
		expectedPerms  []string
		expectedPublic bool
	}{
		{"wildcard rule is reachable", http.MethodDelete, "/admin/orgs/123", []string{"admin:read"}, false},
		{"most specific rule wins", http.MethodGet, "/team/reports", []string{"team:read"}, false},
		{"public rule reported as public", http.MethodGet, "/health", nil, true},
		{"unconfigured path returns nothing", http.MethodGet, "/nope", nil, false},
	}

	config := newTestConfig(t, []EndpointMapping{
		{Path: "/admin/{path:.*}", Methods: []string{"*"}, RequiredPermissions: []string{"admin:read"}},
		{Path: "/team/reports", Methods: []string{"GET"}, RequiredPermissions: []string{"team:read"}},
		{Path: "/health", Methods: []string{"GET"}, Public: true},
	}, nil)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			perms, isPublic := config.GetEndpointPermission(tc.method, tc.path)

			assert.Equal(t, tc.expectedPublic, isPublic)
			assert.Equal(t, tc.expectedPerms, perms)
		})
	}
}

func TestMiddleware_WildcardMethodEnforcement(t *testing.T) {
	testCases := []struct {
		desc         string
		role         string
		expectedCode int
	}{
		{"no role is rejected", "", http.StatusUnauthorized},
		{"insufficient role is rejected", "viewer", http.StatusForbidden},
		{"authorized role is allowed", "admin", http.StatusOK},
	}

	config := newTestConfig(t,
		[]EndpointMapping{
			{Path: "/admin/{path:.*}", Methods: []string{"*"}, RequiredPermissions: []string{"admin:write"}},
		},
		[]RoleDefinition{
			{Name: "admin", Permissions: []string{"admin:write"}},
			{Name: "viewer", Permissions: []string{"admin:read"}},
		},
	)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			reached := false
			handler := Middleware(config)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				reached = true
			}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/orgs/123", http.NoBody)
			if tc.role != "" {
				req.Header.Set("X-User-Role", tc.role)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedCode, rec.Code)
			assert.Equal(t, tc.expectedCode == http.StatusOK, reached)
		})
	}
}

func TestLoadPermissions_InvalidMuxPattern(t *testing.T) {
	testCases := []struct {
		desc      string
		path      string
		expectErr bool
	}{
		{"unparsable regex constraint", "/api/{id:[}", true},
		{"unbalanced parenthesis in constraint", "/api/{id:(}", true},
		{"valid numeric constraint", "/api/{id:[0-9]+}", false},
		{"valid catch-all", "/api/{path:.*}", false},
		{"plain path", "/api/users", false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fileContent := `{
				"roles": [{"name": "admin", "permissions": ["admin:read"]}],
				"endpoints": [{"path": "` + tc.path + `", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
			}`

			path, err := createTestConfigFile("test_pattern_config.json", fileContent)
			require.NoError(t, err)

			defer os.Remove(path)

			config, err := LoadPermissions("test_pattern_config.json", nil, nil, nil)

			if tc.expectErr {
				require.Error(t, err)
				assert.Nil(t, config)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}
