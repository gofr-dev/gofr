package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
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

	tests := []struct {
		name        string
		ctx         context.Context
		expectedErr error
	}{
		{
			name:        "No Metadata",
			ctx:         context.Background(),
			expectedErr: status.Error(codes.Unauthenticated, "missing metadata"),
		},
		{
			name:        "No Authorization Header",
			ctx:         metadata.NewIncomingContext(context.Background(), metadata.MD{}),
			expectedErr: status.Error(codes.Unauthenticated, "missing authorization header"),
		},
		{
			name: "Invalid Format",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Bearer token"},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid authorization header format"),
		},
		{
			name: "Invalid Base64",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Basic invalid-base64"},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid base64 credentials"),
		},
		{
			name: "Invalid Credentials Format",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user"))},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid credentials format"),
		},
		{
			name: "Wrong Password",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user:wrong"))},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid credentials"),
		},
		{
			name: "Wrong User",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("wrong:pass"))},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid credentials"),
		},
		{
			name: "Success",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))},
			}),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := interceptor(tt.ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
				if tt.expectedErr == nil {
					username := ctx.Value(httpMiddleware.Username)
					assert.Equal(t, "user", username)
				}

				return nil, nil
			})
			assert.Equal(t, tt.expectedErr, err)
		})
	}
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
			username := ctx.Value(httpMiddleware.Username)
			assert.Equal(t, "custom", username)

			return nil, nil
		})

		assert.NoError(t, err)
	})

	t.Run("Validator with Datasources Success", func(t *testing.T) {
		validateFunc := func(_ *container.Container, username, password string) bool {
			return username == "validator" && password == "pass"
		}
		interceptor := BasicAuthUnaryInterceptor(BasicAuthProvider{ValidateFuncWithDatasources: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("validator:pass"))},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			username := ctx.Value(httpMiddleware.Username)
			assert.Equal(t, "validator", username)

			return nil, nil
		})

		assert.NoError(t, err)
	})
}

func TestAPIKeyAuthUnaryInterceptor(t *testing.T) {
	keys := []string{"valid-key"}
	interceptor := APIKeyAuthUnaryInterceptor(APIKeyAuthProvider{APIKeys: keys})

	tests := []struct {
		name        string
		ctx         context.Context
		expectedErr error
	}{
		{
			name:        "No Metadata",
			ctx:         context.Background(),
			expectedErr: status.Error(codes.Unauthenticated, "missing metadata"),
		},
		{
			name:        "No API Key Header",
			ctx:         metadata.NewIncomingContext(context.Background(), metadata.MD{}),
			expectedErr: status.Error(codes.Unauthenticated, "missing x-api-key header"),
		},
		{
			name: "Invalid Key",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"x-api-key": []string{"invalid-key"},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid api key"),
		},
		{
			name: "Success",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"x-api-key": []string{"valid-key"},
			}),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := interceptor(tt.ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
				if tt.expectedErr == nil {
					apiKey := ctx.Value(httpMiddleware.APIKey)
					assert.Equal(t, "valid-key", apiKey)
				}

				return nil, nil
			})
			assert.Equal(t, tt.expectedErr, err)
		})
	}
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
			apiKey := ctx.Value(httpMiddleware.APIKey)
			assert.Equal(t, "custom-key", apiKey)

			return nil, nil
		})

		assert.NoError(t, err)
	})

	t.Run("Validator with Datasources Success", func(t *testing.T) {
		validateFunc := func(_ *container.Container, apiKey string) bool {
			return apiKey == "validator-key"
		}
		interceptor := APIKeyAuthUnaryInterceptor(APIKeyAuthProvider{ValidateFuncWithDatasources: validateFunc})
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"x-api-key": []string{"validator-key"},
		})

		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
			apiKey := ctx.Value(httpMiddleware.APIKey)
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

	tests := []struct {
		name        string
		ctx         context.Context
		expectedErr error
	}{
		{
			name:        "No Metadata",
			ctx:         context.Background(),
			expectedErr: status.Error(codes.Unauthenticated, "missing metadata"),
		},
		{
			name:        "No Authorization Header",
			ctx:         metadata.NewIncomingContext(context.Background(), metadata.MD{}),
			expectedErr: status.Error(codes.Unauthenticated, "missing authorization header"),
		},
		{
			name: "Invalid Format",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Token " + validToken},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "invalid authorization header format"),
		},
		{
			name: "Invalid Token",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Bearer invalid-token"},
			}),
			expectedErr: status.Error(codes.Unauthenticated, "jwt expected"),
		},
		{
			name: "Success",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Bearer " + validToken},
			}),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := interceptor(tt.ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
				// Check if claims are in context
				if tt.expectedErr == nil {
					claims := ctx.Value(httpMiddleware.JWTClaim)
					assert.NotNil(t, claims)
				}

				return nil, nil
			})
			if tt.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
