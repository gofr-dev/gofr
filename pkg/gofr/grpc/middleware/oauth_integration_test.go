package middleware

import (
	"context"
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
	"google.golang.org/grpc/metadata"

	auth "gofr.dev/pkg/gofr/http/middleware"
)

func TestOAuthIntegration_MockJWKS(t *testing.T) {
	// 1. Setup Mock JWKS Server
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := &privateKey.PublicKey
	keyID := "valid-kid"

	nBase64 := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	eBase64 := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())

	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": keyID,
				"n":   nBase64,
				"e":   eBase64,
				"use": "sig",
				"alg": "RS256",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// 2. Setup Interceptor with mock PublicKeyProvider
	provider := &mockKeyProvider{key: publicKey}

	interceptor := OAuthUnaryInterceptor(provider)

	// 3. Generate a valid token
	claims := jwt.MapClaims{
		"sub":  "test-user",
		"role": "admin",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	tokenString, _ := token.SignedString(privateKey)

	// 4. Test Success Path
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"authorization": []string{"Bearer " + tokenString},
	})

	_, err := interceptor(ctx, nil, nil, func(handlerCtx context.Context, _ any) (any, error) {
		// Verify claims are injected
		injectedClaims, ok := handlerCtx.Value(auth.JWTClaim).(jwt.MapClaims)
		assert.True(t, ok)
		assert.Equal(t, "test-user", injectedClaims["sub"])
		assert.Equal(t, "admin", injectedClaims["role"])

		return nil, nil
	})

	require.NoError(t, err)

	// 5. Test Failure Path (Invalid Token)
	ctxInvalid := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"authorization": []string{"Bearer invalid.token.here"},
	})

	_, err = interceptor(ctxInvalid, nil, nil, func(_ context.Context, _ any) (any, error) {
		return nil, nil
	})

	assert.Error(t, err)
}
