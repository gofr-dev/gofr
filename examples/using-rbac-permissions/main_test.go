package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
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

func TestRBACPermissions(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksServer := newJWKSServer(t, key)
	t.Setenv("JWKS_URL", jwksServer.URL+"/.well-known/jwks.json")
	t.Setenv("TOKEN_ISSUER", issuer)

	serverConfigs := testutil.NewServerConfigs(t)

	go main()

	testutil.WaitForHTTPServer(t, serverConfigs.HTTPHost)

	sign := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = keyID

		signed, signErr := token.SignedString(key)
		require.NoError(t, signErr)

		return signed
	}

	exp := time.Now().Add(time.Hour).Unix()
	client := &http.Client{Timeout: time.Second}

	call := func(method string, claims jwt.MapClaims) int {
		req, reqErr := http.NewRequestWithContext(t.Context(), method, serverConfigs.HTTPHost+"/orders", http.NoBody)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+sign(claims))

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		resp.Body.Close()

		return resp.StatusCode
	}

	readScope := jwt.MapClaims{"scope": "orders:read", "aud": "orders-api", "exp": exp, "iss": issuer}

	// The OAuth middleware fetches the signing keys in the background; wait until it has them.
	require.Eventually(t, func() bool { return call(http.MethodGet, readScope) == http.StatusOK },
		5*time.Second, 50*time.Millisecond)

	testCases := []struct {
		desc       string
		method     string
		claims     jwt.MapClaims
		wantStatus int
	}{
		{desc: "scope holds the permission", method: http.MethodGet, claims: readScope, wantStatus: http.StatusOK},
		{
			desc: "one of several scopes grants", method: http.MethodPost,
			claims:     jwt.MapClaims{"scope": "orders:read orders:write", "aud": "orders-api", "exp": exp, "iss": issuer},
			wantStatus: http.StatusCreated,
		},
		{desc: "scope lacks the permission", method: http.MethodPost, claims: readScope, wantStatus: http.StatusForbidden},
		{
			desc: "token minted for another API", method: http.MethodGet,
			claims:     jwt.MapClaims{"scope": "orders:read", "aud": "billing-api", "exp": exp, "iss": issuer},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "token without an expiry", method: http.MethodGet,
			claims:     jwt.MapClaims{"scope": "orders:read", "aud": "orders-api", "iss": issuer},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "token from another issuer", method: http.MethodGet,
			claims:     jwt.MapClaims{"scope": "orders:read", "aud": "orders-api", "exp": exp, "iss": "https://evil-idp/"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "token without an issuer", method: http.MethodGet,
			claims:     jwt.MapClaims{"scope": "orders:read", "aud": "orders-api", "exp": exp},
			wantStatus: http.StatusUnauthorized,
		},
		{
			desc: "token without a scope holds nothing", method: http.MethodGet,
			claims:     jwt.MapClaims{"aud": "orders-api", "exp": exp, "iss": issuer},
			wantStatus: http.StatusForbidden,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantStatus, call(tc.method, tc.claims), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
