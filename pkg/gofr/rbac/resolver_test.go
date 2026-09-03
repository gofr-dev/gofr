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

	// A constrained variable admits fewer paths than a free one, so it has to outrank it -
	// otherwise these two score equally and the sort falls through to declaration order.
	free := EndpointMapping{Path: "/users/{id}", Methods: []string{"*"}, RequiredPermissions: []string{"users:read"}}
	constrained := EndpointMapping{Path: "/users/{id:[0-9]+}", Methods: []string{"*"}, RequiredPermissions: []string{"users:write"}}

	// Same depth, differing only in one segment, and the more specific one is declared last: these
	// go red if segments stop being compared position by position, where a case whose patterns
	// differ in length or that a literal path resolves by exact lookup would still pass.
	varPrefix := EndpointMapping{Path: "/{scope}/orgs/{org_id}", Methods: []string{"*"}, RequiredPermissions: []string{"scope:read"}}
	varTail := EndpointMapping{Path: "/admin/{section}/{org_id}", Methods: []string{"*"}, RequiredPermissions: []string{"admin:list"}}
	litTail := EndpointMapping{Path: "/admin/orgs/{org_id}", Methods: []string{"*"}, RequiredPermissions: []string{"orgs:read"}}

	testCases := []struct {
		desc      string
		endpoints []EndpointMapping
		path      string
		expected  string
	}{
		{"narrow wins over broad", []EndpointMapping{broad, narrow}, "/admin/orgs/123", "/admin/orgs/{org_id}"},
		{"declaration order is irrelevant", []EndpointMapping{narrow, broad}, "/admin/orgs/123", "/admin/orgs/{org_id}"},
		{"literal wins over param", []EndpointMapping{broad, narrow, literal}, "/admin/orgs/global", "/admin/orgs/global"},
		{"literal segment wins over a variable in the same position", []EndpointMapping{varPrefix, litTail},
			"/admin/orgs/123", "/admin/orgs/{org_id}"},
		{"the first differing segment decides", []EndpointMapping{varPrefix, varTail}, "/admin/orgs/123", "/admin/{section}/{org_id}"},
		{"broad wins when it is the only match", []EndpointMapping{broad, narrow}, "/admin/settings", "/admin/{path:.*}"},
		{"constrained variable wins over free one", []EndpointMapping{free, constrained}, "/users/42", "/users/{id:[0-9]+}"},
		{"constrained variable wins in either declaration order", []EndpointMapping{constrained, free}, "/users/42", "/users/{id:[0-9]+}"},
		{"free variable still matches what the constraint rejects", []EndpointMapping{constrained, free}, "/users/me", "/users/{id}"},
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

func TestLoadPermissions_UncompilablePatternIsNonFatal(t *testing.T) {
	testCases := []struct {
		desc      string
		path      string
		expectLog bool
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

			logger := &mockLogger{}

			config, err := LoadPermissions("test_pattern_config.json", logger, nil, nil)

			// A pattern mux cannot compile does not stop the app from booting - it is logged loudly
			// instead, because the endpoint it governs will silently never match.
			require.NoError(t, err)
			require.NotNil(t, config)

			if tc.expectLog {
				require.Len(t, logger.errorLogs, 1)
				assert.Contains(t, logger.errorLogs[0], tc.path)
				assert.Contains(t, logger.errorLogs[0], "NOT enforced")
			} else {
				assert.Empty(t, logger.errorLogs)
			}
		})
	}
}

func TestLoadPermissions_UncompilablePatternWithoutLogger(t *testing.T) {
	fileContent := `{
		"roles": [{"name": "admin", "permissions": ["admin:read"]}],
		"endpoints": [{"path": "/api/{id:[}", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
	}`

	path, err := createTestConfigFile("test_pattern_nolog_config.json", fileContent)
	require.NoError(t, err)

	defer os.Remove(path)

	config, err := LoadPermissions("test_pattern_nolog_config.json", nil, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, config)
}

func TestLoadPermissions_UnbalancedBracesStillFails(t *testing.T) {
	fileContent := `{
		"roles": [{"name": "admin", "permissions": ["admin:read"]}],
		"endpoints": [{"path": "/api/{id}}", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
	}`

	path, err := createTestConfigFile("test_pattern_braces_config.json", fileContent)
	require.NoError(t, err)

	defer os.Remove(path)

	config, err := LoadPermissions("test_pattern_braces_config.json", nil, nil, nil)

	require.ErrorIs(t, err, errUnbalancedBraces)
	assert.Nil(t, config)
}

func TestPathSpecificity(t *testing.T) {
	testCases := []struct {
		desc     string
		pattern  string
		expected []int
	}{
		{"empty pattern has no segments", "", nil},
		{"literal segments", "/admin/orgs/global", []int{segLiteral, segLiteral, segLiteral}},
		{"free variable", "/users/{id}", []int{segLiteral, segVariable}},
		{"constrained variable", "/users/{id:[0-9]+}", []int{segLiteral, segConstrained}},
		{"constrained variable with a quantifier", "/users/{id:[0-9]{2,3}}", []int{segLiteral, segConstrained}},
		{"documented catch-all", "/admin/{path:.*}", []int{segLiteral, segCatchAll}},
		{"one-or-more catch-all", "/admin/{path:.+}", []int{segLiteral, segCatchAll}},

		// A "/" inside the constraint must not be treated as a segment separator: splitting on it
		// would leave three fragments that each look literal, scoring the loosest pattern in the
		// config as the most specific one.
		{"constraint admitting a slash spans segments", "/files/{path:[a-z/]+}", []int{segLiteral, segCatchAll}},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expected, pathSpecificity(tc.pattern))
		})
	}
}

func TestCompareSpecificity(t *testing.T) {
	testCases := []struct {
		desc         string
		a, b         []int
		expectAFirst bool
		expectTie    bool
	}{
		{"literal beats constrained in the same position", []int{segLiteral, segLiteral}, []int{segLiteral, segConstrained}, true, false},
		{"constrained beats free", []int{segLiteral, segConstrained}, []int{segLiteral, segVariable}, true, false},
		{"free beats catch-all", []int{segLiteral, segVariable}, []int{segLiteral, segCatchAll}, true, false},
		{"the first differing segment decides, not later ones", []int{segCatchAll, segLiteral}, []int{segLiteral, segCatchAll}, false, false},
		{"a longer path wins when one is a prefix of the other", []int{segLiteral, segLiteral}, []int{segLiteral}, true, false},
		{"identical vectors tie", []int{segLiteral, segVariable}, []int{segLiteral, segVariable}, false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := compareSpecificity(tc.a, tc.b)

			if tc.expectTie {
				assert.Equal(t, 0, got)
				return
			}

			assert.Equal(t, tc.expectAFirst, got > 0)
			// The comparison has to be antisymmetric, or sort.SliceStable's ordering is undefined.
			assert.Equal(t, tc.expectAFirst, compareSpecificity(tc.b, tc.a) < 0)
		})
	}
}

func TestGetEndpointForRequest_DuplicateDeclarationLastWins(t *testing.T) {
	public := EndpointMapping{Path: "/admin/reports", Methods: []string{"GET"}, Public: true}
	protected := EndpointMapping{Path: "/admin/reports", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}}

	testCases := []struct {
		desc           string
		endpoints      []EndpointMapping
		expectedPublic bool
		expectedPerms  []string
	}{
		// The public flag and the permission list live in maps of their own, so overwriting the
		// endpoint alone would leave the earlier declaration's flag behind - and a stale public
		// flag serves the route unauthenticated.
		{"protected declared last is enforced", []EndpointMapping{public, protected}, false, []string{"admin:read"}},
		{"public declared last is public", []EndpointMapping{protected, public}, true, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := newTestConfig(t, tc.endpoints, nil)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/reports", http.NoBody)
			endpoint, isPublic := getEndpointForRequest(req, config)

			require.NotNil(t, endpoint)
			assert.Equal(t, tc.expectedPublic, isPublic)
			assert.Equal(t, tc.expectedPerms, endpoint.RequiredPermissions)

			perms, isPublic := config.GetEndpointPermission(http.MethodGet, "/admin/reports")
			assert.Equal(t, tc.expectedPublic, isPublic)
			assert.Equal(t, tc.expectedPerms, perms)
		})
	}
}

func TestGetEndpointForRequest_ProtectedBeatsPublicOnTie(t *testing.T) {
	public := EndpointMapping{Path: "/{a}/{b}", Methods: []string{"GET"}, Public: true}
	protected := EndpointMapping{Path: "/{x}/{y}", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read"}}

	testCases := []struct {
		desc      string
		endpoints []EndpointMapping
	}{
		{"public declared first", []EndpointMapping{public, protected}},
		{"protected declared first", []EndpointMapping{protected, public}},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := newTestConfig(t, tc.endpoints, nil)

			for range 100 {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/foo/bar", http.NoBody)
				endpoint, isPublic := getEndpointForRequest(req, config)

				require.NotNil(t, endpoint)
				assert.False(t, isPublic, "a tie must not resolve to the public rule")
				assert.Equal(t, "/{x}/{y}", endpoint.Path)
			}
		})
	}
}

func TestGetEndpointForRequest_SlashConstraintDoesNotOutrankNarrowRule(t *testing.T) {
	// "{path:[a-z/]+}" spans segments exactly as "{path:.*}" does. Scored naively it reads as
	// three literal segments and outranks the narrow rule below, which is the fail-open direction
	// when the broad rule is the more permissive one.
	broad := EndpointMapping{Path: "/files/{path:[a-z/]+}", Methods: []string{"*"}, RequiredPermissions: []string{"files:read"}}
	narrow := EndpointMapping{Path: "/files/private/{name}", Methods: []string{"*"}, RequiredPermissions: []string{"files:admin"}}

	for _, endpoints := range [][]EndpointMapping{{broad, narrow}, {narrow, broad}} {
		config := newTestConfig(t, endpoints, nil)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/files/private/secret", http.NoBody)
		endpoint, _ := getEndpointForRequest(req, config)

		require.NotNil(t, endpoint)
		assert.Equal(t, "/files/private/{name}", endpoint.Path)
	}
}
