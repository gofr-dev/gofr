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

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

type staticKeyProvider struct{ key *rsa.PublicKey }

func (p staticKeyProvider) Get(string) *rsa.PublicKey { return p.key }

// grantsBySubject reads the claims the OAuth middleware verified, the way an application extractor
// keyed on the token's subject does.
func grantsBySubject(_ *container.Container, r *http.Request) ([]string, error) {
	claims, ok := r.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
	if !ok {
		return nil, ErrRoleNotFound
	}

	if claims["sub"] == "alice" {
		return []string{"orders:write"}, nil
	}

	return nil, nil
}

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
		extractor  bool // permissions come from grantsBySubject
		claims     jwt.MapClaims
		wantStatus int
	}{
		{
			desc:       "permissions mode: scope grants",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read orders:write", "exp": exp},
			wantStatus: http.StatusOK,
		},
		{
			desc:       "permissions mode: scope lacks the permission",
			config:     Config{PermissionsClaimPath: "scope", Endpoints: endpoints},
			claims:     jwt.MapClaims{"scope": "orders:read", "exp": exp},
			wantStatus: http.StatusForbidden,
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
		{
			desc:       "extractor mode: the verified subject is granted",
			config:     Config{Endpoints: endpoints},
			extractor:  true,
			claims:     jwt.MapClaims{"sub": "alice", "exp": exp},
			wantStatus: http.StatusOK,
		},
		{
			desc:       "extractor mode: another subject is denied",
			config:     Config{Endpoints: endpoints},
			extractor:  true,
			claims:     jwt.MapClaims{"sub": "bob", "exp": exp},
			wantStatus: http.StatusForbidden,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := tc.config
			if tc.extractor {
				WithExtractor(grantsBySubject, nil, 0)(&cfg)
			}

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
