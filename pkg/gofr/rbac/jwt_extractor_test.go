package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/http/middleware"
)

func TestNewJWTRoleExtractor(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("role")
		assert.NotNil(t, extractor)
	})

	t.Run("EmptyClaim", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("")
		assert.NotNil(t, extractor)
	})
}

func TestJWTRoleExtractorProvider_ExtractRole(t *testing.T) {
	t.Run("SimpleClaim", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("role")

		claims := jwt.MapClaims{
			"role": "admin",
		}

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

		role, err := extractor.ExtractRole(req)
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})

	t.Run("ArrayNotation", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("roles[0]")

		claims := jwt.MapClaims{
			"roles": []any{"admin", "editor"},
		}

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

		role, err := extractor.ExtractRole(req)
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})

	t.Run("NestedClaim", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("permissions.role")

		claims := jwt.MapClaims{
			"permissions": map[string]any{
				"role": "admin",
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

		role, err := extractor.ExtractRole(req)
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})

	t.Run("NoJWTClaims", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("role")

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		// No JWT claims in context

		role, err := extractor.ExtractRole(req)
		require.Error(t, err)
		assert.Empty(t, role)
	})

	t.Run("ClaimNotFound", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("nonexistent")

		claims := jwt.MapClaims{
			"role": "admin",
		}

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

		role, err := extractor.ExtractRole(req)
		require.Error(t, err)
		assert.Empty(t, role)
	})

	t.Run("WithArgs", func(t *testing.T) {
		extractor := NewJWTRoleExtractor("role")

		claims := jwt.MapClaims{
			"role": "admin",
		}

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

		// ExtractRole should ignore args parameter
		role, err := extractor.ExtractRole(req, "arg1", "arg2")
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})
}
