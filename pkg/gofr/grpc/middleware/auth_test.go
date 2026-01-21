package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	auth "gofr.dev/pkg/gofr/http/middleware"
)

type mockKeyProvider struct {
	key *rsa.PublicKey
}

func (m *mockKeyProvider) Get(kid string) *rsa.PublicKey {
	if kid == "valid-kid" {
		return m.key
	}

	return nil
}

func TestBasicAuthUnaryInterceptor(t *testing.T) {
	users := map[string]string{"user": "pass"}
	interceptor := BasicAuthUnaryInterceptor(BasicAuthProvider{Users: users})

	t.Run("No Metadata", func(t *testing.T) {
		ctx := context.Background()
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing metadata"), err)
	})

	t.Run("No Authorization Header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing authorization header"), err)
	})

	t.Run("Invalid Format", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer token"},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid authorization header format"), err)
	})

	t.Run("Invalid Base64", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic invalid-base64"},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid base64 credentials"), err)
	})

	t.Run("Invalid Credentials Format", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user"))},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid credentials format"), err)
	})

	t.Run("Wrong Password", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user:wrong"))},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid credentials"), err)
	})

	t.Run("Wrong User", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("wrong:pass"))},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid credentials"), err)
	})

	t.Run("Success", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))},
		})
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			username := ctx.Value(auth.Username)
			assert.Equal(t, "user", username)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

func TestBasicAuthUnaryInterceptor_Validator(t *testing.T) {
	t.Run("Custom Validation Function Success", func(t *testing.T) {
		validateFunc := func(username, password string) bool {
			return username == "custom" && password == "pass"
		}
		interceptor := BasicAuthUnaryInterceptor(BasicAuthProvider{ValidateFunc: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("custom:pass"))},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			username := ctx.Value(auth.Username)
			assert.Equal(t, "custom", username)

			return nil, nil
		})

		assert.NoError(t, err)
	})

	t.Run("Validator with Datasources Success", func(t *testing.T) {
		validateFunc := func(_ *container.Container, username, password string) bool {
			return username == "validator" && password == "pass"
		}
		interceptor := BasicAuthUnaryInterceptor(BasicAuthProvider{ValidateFuncWithDatasource: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("validator:pass"))},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			username := ctx.Value(auth.Username)
			assert.Equal(t, "validator", username)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

func TestAPIKeyAuthUnaryInterceptor(t *testing.T) {
	keys := []string{"valid-key"}
	interceptor := APIKeyAuthUnaryInterceptor(APIKeyAuthProvider{APIKeys: keys})

	t.Run("No Metadata", func(t *testing.T) {
		ctx := context.Background()
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing metadata"), err)
	})

	t.Run("No API Key Header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing x-api-key header"), err)
	})

	t.Run("Invalid Key", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"x-api-key": []string{"invalid-key"},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid api key"), err)
	})

	t.Run("Success", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"x-api-key": []string{"valid-key"},
		})
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			apiKey := ctx.Value(auth.APIKey)
			assert.Equal(t, "valid-key", apiKey)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

func TestAPIKeyAuthUnaryInterceptor_Validator(t *testing.T) {
	t.Run("Custom Validation Function Success", func(t *testing.T) {
		validateFunc := func(apiKey string) bool {
			return apiKey == "custom-key"
		}
		interceptor := APIKeyAuthUnaryInterceptor(APIKeyAuthProvider{ValidateFunc: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"x-api-key": []string{"custom-key"},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			apiKey := ctx.Value(auth.APIKey)
			assert.Equal(t, "custom-key", apiKey)

			return nil, nil
		})

		assert.NoError(t, err)
	})

	t.Run("Validator with Datasources Success", func(t *testing.T) {
		validateFunc := func(_ *container.Container, apiKey string) bool {
			return apiKey == "validator-key"
		}
		interceptor := APIKeyAuthUnaryInterceptor(APIKeyAuthProvider{ValidateFuncWithDatasource: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"x-api-key": []string{"validator-key"},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			apiKey := ctx.Value(auth.APIKey)
			assert.Equal(t, "validator-key", apiKey)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

func TestOAuthUnaryInterceptor(t *testing.T) {
	// Generate RSA key
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := &privateKey.PublicKey

	provider := &mockKeyProvider{key: publicKey}
	interceptor := OAuthUnaryInterceptor(provider)

	// Create valid token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user",
	})
	token.Header["kid"] = "valid-kid"
	validToken, _ := token.SignedString(privateKey)

	t.Run("No Metadata", func(t *testing.T) {
		ctx := context.Background()
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing metadata"), err)
	})

	t.Run("No Authorization Header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "missing authorization header"), err)
	})

	t.Run("Invalid Format", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Token " + validToken},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "invalid authorization header format"), err)
	})

	t.Run("Invalid Token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer invalid-token"},
		})
		_, err := interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			return nil, nil
		})

		assert.Equal(t, status.Error(codes.Unauthenticated, "jwt expected"), err)
	})

	t.Run("Success", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer " + validToken},
		})
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			claims := ctx.Value(auth.JWTClaim)
			assert.NotNil(t, claims)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestWrappedStream(t *testing.T) {
	ctx := context.Background()
	m := &mockServerStream{ctx: ctx}
	newCtx := context.WithValue(ctx, auth.Username, "user")
	w := &wrappedStream{ServerStream: m, ctx: newCtx}

	assert.Equal(t, newCtx, w.Context())
}
