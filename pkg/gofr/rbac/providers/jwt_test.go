package providers

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
	extractor := NewJWTRoleExtractor("role")
	assert.NotNil(t, extractor)
	assert.Equal(t, "role", extractor.RoleClaim)

	// Test default claim name
	extractor2 := NewJWTRoleExtractor("")
	assert.NotNil(t, extractor2)
	assert.Equal(t, "role", extractor2.RoleClaim)
}

func TestJWTRoleExtractor_ExtractRole_SimpleClaim(t *testing.T) {
	extractor := NewJWTRoleExtractor("role")

	claims := jwt.MapClaims{
		"role": "admin",
		"sub":  "user123",
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestJWTRoleExtractor_ExtractRole_ArrayNotation(t *testing.T) {
	extractor := NewJWTRoleExtractor("roles[0]")

	claims := jwt.MapClaims{
		"roles": []any{"admin", "editor"},
		"sub":   "user123",
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestJWTRoleExtractor_ExtractRole_NestedClaim(t *testing.T) {
	extractor := NewJWTRoleExtractor("permissions.role")

	claims := jwt.MapClaims{
		"permissions": map[string]any{
			"role": "admin",
		},
		"sub": "user123",
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestJWTRoleExtractor_ExtractRole_NoJWT(t *testing.T) {
	extractor := NewJWTRoleExtractor("role")

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	// No JWT claims in context

	role, err := extractor.ExtractRole(req)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrJWTNotEnabled)
	assert.Empty(t, role)
}

func TestJWTRoleExtractor_ExtractRole_ClaimNotFound(t *testing.T) {
	extractor := NewJWTRoleExtractor("nonexistent")

	claims := jwt.MapClaims{
		"role": "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrRoleClaimNotFound)
	assert.Empty(t, role)
}

func TestJWTRoleExtractor_ExtractRole_NonStringValue(t *testing.T) {
	extractor := NewJWTRoleExtractor("role")

	claims := jwt.MapClaims{
		"role": 123, // Non-string value
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.NoError(t, err) // Should convert to string
	assert.Equal(t, "123", role)
}

func TestJWTRoleExtractor_ExtractRole_ArrayIndexOutOfBounds(t *testing.T) {
	extractor := NewJWTRoleExtractor("roles[5]")

	claims := jwt.MapClaims{
		"roles": []any{"admin", "editor"},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.Error(t, err)
	assert.Empty(t, role)
}

func TestJWTRoleExtractor_ExtractRole_InvalidArrayIndex(t *testing.T) {
	extractor := NewJWTRoleExtractor("roles[invalid]")

	claims := jwt.MapClaims{
		"roles": []any{"admin", "editor"},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.Error(t, err)
	assert.Empty(t, role)
}

func TestJWTRoleExtractor_ExtractRole_DeeplyNested(t *testing.T) {
	extractor := NewJWTRoleExtractor("user.permissions.role")

	claims := jwt.MapClaims{
		"user": map[string]any{
			"permissions": map[string]any{
				"role": "admin",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.JWTClaim, claims))

	role, err := extractor.ExtractRole(req)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestExtractClaimValue_SimpleKey(t *testing.T) {
	claims := jwt.MapClaims{
		"role": "admin",
	}

	value, err := extractClaimValue(claims, "role")
	require.NoError(t, err)
	assert.Equal(t, "admin", value)
}

func TestExtractClaimValue_ArrayNotation(t *testing.T) {
	claims := jwt.MapClaims{
		"roles": []any{"admin", "editor", "viewer"},
	}

	value, err := extractClaimValue(claims, "roles[1]")
	require.NoError(t, err)
	assert.Equal(t, "editor", value)
}

func TestExtractClaimValue_DotNotation(t *testing.T) {
	claims := jwt.MapClaims{
		"user": map[string]any{
			"role": "admin",
		},
	}

	value, err := extractClaimValue(claims, "user.role")
	require.NoError(t, err)
	assert.Equal(t, "admin", value)
}

func TestExtractClaimValue_NotFound(t *testing.T) {
	claims := jwt.MapClaims{
		"other": "value",
	}

	value, err := extractClaimValue(claims, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, value)
}

func TestExtractClaimValue_EmptyPath(t *testing.T) {
	claims := jwt.MapClaims{
		"role": "admin",
	}

	value, err := extractClaimValue(claims, "")
	require.Error(t, err)
	assert.Nil(t, value)
}
