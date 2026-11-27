package main

import (
	"context"
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

	"gofr.dev/pkg/gofr/http/middleware"
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

func TestLivePermissions(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksServer := newJWKSServer(t, key)
	t.Setenv("JWKS_URL", jwksServer.URL+"/.well-known/jwks.json")
	t.Setenv("TOKEN_ISSUER", issuer)

	serverConfigs := testutil.NewServerConfigs(t)

	go main()

	testutil.WaitForHTTPServer(t, serverConfigs.HTTPHost)

	exp := time.Now().Add(time.Hour).Unix()

	// token signs a valid token for subject; an empty subject leaves "sub" out. Any permissions are put
	// in the token's own permissions claim, which the store's answer must replace.
	token := func(subject string, permissions ...any) string {
		claims := jwt.MapClaims{"aud": "orders-api", "exp": exp, "iss": issuer}
		if subject != "" {
			claims["sub"] = subject
		}

		if len(permissions) > 0 {
			claims[permissionsClaim] = permissions
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

	// What the store holds decides each request, per subject.
	assert.Equal(t, http.StatusCreated, call(http.MethodPost, "/orders", alice), "alice holds orders:write")
	assert.Equal(t, http.StatusForbidden, call(http.MethodPost, "/orders", token("bob")), "bob does not hold orders:write")
	assert.Equal(t, http.StatusForbidden, call(http.MethodGet, "/orders", token("carol")), "an unknown subject holds nothing")
	assert.Equal(t, http.StatusForbidden, call(http.MethodGet, "/orders", token("")), "a token without a subject holds nothing")

	// Permissions the token carries itself are never trusted: the store's answer replaces them, and a
	// token without a subject has them removed.
	assert.Equal(t, http.StatusForbidden, call(http.MethodPost, "/orders", token("bob", "orders:write")),
		"bob's token claims orders:write, but the store does not grant it")
	assert.Equal(t, http.StatusForbidden, call(http.MethodPost, "/orders", token("", "orders:write")),
		"a token without a subject must not be authorized from its own permissions claim")

	// Revoking in the store denies the next request with the SAME token; granting restores it.
	store.revoke("alice", "orders:write")

	assert.Equal(t, http.StatusForbidden, call(http.MethodPost, "/orders", alice), "a revoked permission must be denied")
	assert.Equal(t, http.StatusOK, call(http.MethodGet, "/orders", alice), "a permission not revoked must still allow")

	store.grant("alice", "orders:write")

	assert.Equal(t, http.StatusCreated, call(http.MethodPost, "/orders", alice), "a re-granted permission must allow again")

	// A store outage fails closed with a 503 on every protected route.
	store.setFailing(true)
	t.Cleanup(func() { store.setFailing(false) })

	assert.Equal(t, http.StatusServiceUnavailable, call(http.MethodGet, "/orders", alice))
	assert.Equal(t, http.StatusServiceUnavailable, call(http.MethodPost, "/orders", alice))

	// Public and built-in routes never reach the store, so they still answer during the outage.
	before := store.lookups.Load()

	assert.Equal(t, http.StatusOK, call(http.MethodGet, "/status", alice))
	assert.Equal(t, http.StatusOK, call(http.MethodGet, "/.well-known/alive", ""))
	assert.Equal(t, before, store.lookups.Load(), "public and /.well-known/ routes must not call the store")
}

// TestLivePermissionsTimeout checks that a lookup that outlives its timeout is answered with a 503
// and never reaches the next handler.
func TestLivePermissionsTimeout(t *testing.T) {
	slow := func(ctx context.Context, _ string) ([]string, error) {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/orders", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, jwt.MapClaims{"sub": "alice"}))
	rec := httptest.NewRecorder()

	livePermissions(slow, 20*time.Millisecond)(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.False(t, reached)
}

