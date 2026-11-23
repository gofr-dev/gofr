package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

const (
	keyID  = "test-key"
	issuer = "https://auth.example.com/"
)

// newJWKSServer serves the public half of key as a JWKS, the way an identity provider does.
func newJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()

	jwks := map[string]any{"keys": []map[string]string{{
		"kid": keyID,
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// TestConfigKeepsRouteCheckFail checks that configs/.env keeps GOFR_RBAC_ROUTE_CHECK=fail, the setting
// that makes a copy of this example safe: without it, a route added with no rule is served with no
// authorization and startup only logs an error. The route check itself is tested in pkg/gofr
// (TestRun_RBACRouteCheckFailExitsNonZero).
func TestConfigKeepsRouteCheckFail(t *testing.T) {
	env, err := os.ReadFile("configs/.env")
	require.NoError(t, err)

	assert.Contains(t, string(env), "\nGOFR_RBAC_ROUTE_CHECK=fail\n")
}

func TestRBACExtractor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksServer := newJWKSServer(t, key)
	t.Setenv("JWKS_URL", jwksServer.URL+"/.well-known/jwks.json")
	t.Setenv("TOKEN_ISSUER", issuer)

	serverConfigs := testutil.NewServerConfigs(t)

	go main()

	testutil.WaitForHTTPServer(t, serverConfigs.HTTPHost)

	exp := time.Now().Add(time.Hour).Unix()

	// token signs a valid token for subject; an empty subject leaves "sub" out.
	token := func(subject string) string {
		claims := jwt.MapClaims{"aud": "orders-api", "exp": exp, "iss": issuer}
		if subject != "" {
			claims["sub"] = subject
		}

		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = keyID

		signed, signErr := tok.SignedString(key)
		require.NoError(t, signErr)

		return signed
	}

	client := &http.Client{Timeout: 3 * time.Second}

	call := func(method, path, bearer string) int {
		req, reqErr := http.NewRequestWithContext(t.Context(), method, serverConfigs.HTTPHost+path, http.NoBody)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+bearer)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		resp.Body.Close()

		return resp.StatusCode
	}

	alice := token("alice")

	// The OAuth middleware fetches the signing keys in the background; wait until it has them.
	require.Eventually(t, func() bool { return call(http.MethodGet, "/orders", alice) == http.StatusOK },
		5*time.Second, 50*time.Millisecond)

	testCases := []struct {
		desc       string
		method     string
		path       string
		bearer     string
		wantStatus int
	}{
		{desc: "a granted permission allows the request", method: http.MethodPost, path: "/orders",
			bearer: alice, wantStatus: http.StatusCreated},
		{desc: "a permission the store does not grant is denied", method: http.MethodPost, path: "/orders",
			bearer: token("bob"), wantStatus: http.StatusForbidden},
		{desc: "a subject the store does not know holds nothing", method: http.MethodGet, path: "/orders",
			bearer: token("carol"), wantStatus: http.StatusForbidden},
		{desc: "a token without a subject is unauthenticated", method: http.MethodGet, path: "/orders",
			bearer: token(""), wantStatus: http.StatusUnauthorized},
		{desc: "a public route skips the extractor", method: http.MethodGet, path: "/status",
			bearer: token("carol"), wantStatus: http.StatusOK},
	}

	for i, tc := range testCases {
		assert.Equal(t, tc.wantStatus, call(tc.method, tc.path, tc.bearer), "TEST[%d], Failed.\n%s", i, tc.desc)
	}

	// Revoking in the store denies the next request, though alice's token is still valid.
	store.revoke("alice", "orders:write")

	assert.Equal(t, http.StatusForbidden, call(http.MethodPost, "/orders", alice), "revoked permission must be denied")
	assert.Equal(t, http.StatusOK, call(http.MethodGet, "/orders", alice), "a permission not revoked must still allow")
}
