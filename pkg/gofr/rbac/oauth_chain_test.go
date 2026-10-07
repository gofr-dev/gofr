package rbac

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/http/middleware"
)

type staticKeyProvider struct{ key *rsa.PublicKey }

func (p staticKeyProvider) Get(string) *rsa.PublicKey { return p.key }

// TestMiddleware_BehindOAuth runs RBAC behind the real OAuth middleware, so the claims RBAC reads are
// the ones a verified, signed token produces - JSON numbers, []any arrays and all.
func TestMiddleware_BehindOAuth(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	sign := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test"

		signed, signErr := token.SignedString(key)
		require.NoError(t, signErr)

		return signed
	}

	exp := time.Now().Add(time.Hour).Unix()
	endpoints := []EndpointMapping{{Path: "/orders", Methods: []string{"POST"}, RequiredPermissions: []string{"orders:write"}}}

	testCases := []struct {
		desc       string
		config     Config
		claims     jwt.MapClaims
		wantStatus int
	}{
		{
			desc:       "permissions mode: scope grants",
			config:     Config{PermissionsClaimPath: "scope", Audience: []string{"orders-api"}, Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read orders:write", "aud": "orders-api", "exp": exp},
			wantStatus: http.StatusOK,
		},
		{
			desc:       "permissions mode: scope lacks the permission",
			config:     Config{PermissionsClaimPath: "scope", Audience: []string{"orders-api"}, Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read", "aud": "orders-api", "exp": exp},
			wantStatus: http.StatusForbidden,
		},
		{
			desc:       "permissions mode: token for another API",
			config:     Config{PermissionsClaimPath: "scope", Audience: []string{"orders-api"}, Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:write", "aud": []string{"billing-api"}, "exp": exp},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc:       "permissions mode: token without exp",
			config:     Config{PermissionsClaimPath: "scope", Audience: []string{"orders-api"}, Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:write", "aud": "orders-api"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "roles mode: role array grants",
			config: Config{JWTClaimPath: "roles", Endpoints: endpoints, Roles: []RoleDefinition{
				{Name: "viewer", Permissions: []string{"orders:read"}},
				{Name: "writer", Permissions: []string{"orders:write"}},
			}},
			claims:     jwt.MapClaims{"roles": []string{"viewer", "writer"}, "exp": exp},
			wantStatus: http.StatusOK,
		},
		{
			desc: "roles mode: roles[0] still reads the first role only",
			config: Config{JWTClaimPath: "roles[0]", Endpoints: endpoints, Roles: []RoleDefinition{
				{Name: "viewer", Permissions: []string{"orders:read"}},
				{Name: "writer", Permissions: []string{"orders:write"}},
			}},
			claims:     jwt.MapClaims{"roles": []string{"viewer", "writer"}, "exp": exp},
			wantStatus: http.StatusForbidden,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := tc.config
			require.NoError(t, cfg.processUnifiedConfig())

			chain := middleware.OAuth(staticKeyProvider{key: &key.PublicKey})(
				Middleware(&cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/orders", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+sign(tc.claims))

			w := httptest.NewRecorder()
			chain.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
