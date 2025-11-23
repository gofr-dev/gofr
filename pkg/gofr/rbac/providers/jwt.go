package providers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr/http/middleware"
)

var (
	// ErrJWTNotEnabled is returned when JWT role extraction is attempted but OAuth is not enabled.
	ErrJWTNotEnabled = errors.New("JWT/OAuth middleware not enabled")

	// ErrRoleClaimNotFound is returned when the specified role claim is not found in JWT.
	ErrRoleClaimNotFound = errors.New("role claim not found in JWT")

	ErrEmptyClaimPath        = errors.New("empty claim path")
	ErrInvalidArrayNotation  = errors.New("invalid array notation")
	ErrInvalidArrayIndex     = errors.New("invalid array index")
	ErrClaimKeyNotFound      = errors.New("claim key not found")
	ErrClaimNotArray         = errors.New("claim value is not an array")
	ErrArrayIndexOutOfBounds = errors.New("array index out of bounds")
	ErrClaimPathNotFound     = errors.New("claim path not found")
	ErrInvalidClaimStructure = errors.New("invalid claim structure")
)

// JWTRoleExtractor extracts role from JWT claims stored in request context.
// It works with GoFr's OAuth middleware which stores JWT claims in context.
type JWTRoleExtractor struct {
	// RoleClaim is the path to the role in JWT claims
	// Examples:
	//   - "role" for simple claim: {"role": "admin"}
	//   - "roles[0]" for array: {"roles": ["admin", "user"]}
	//   - "permissions.role" for nested: {"permissions": {"role": "admin"}}
	RoleClaim string
}

// NewJWTRoleExtractor creates a new JWT role extractor.
func NewJWTRoleExtractor(roleClaim string) *JWTRoleExtractor {
	if roleClaim == "" {
		roleClaim = "role" // Default claim name
	}

	return &JWTRoleExtractor{
		RoleClaim: roleClaim,
	}
}

// ExtractRole extracts the role from JWT claims in the request context.
// It expects the OAuth middleware to have already validated the JWT and stored claims.
func (e *JWTRoleExtractor) ExtractRole(req *http.Request, _ ...any) (string, error) {
	// Get JWT claims from context (set by OAuth middleware)
	claims, ok := req.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
	if !ok || claims == nil {
		return "", ErrJWTNotEnabled
	}

	// Extract role using the configured claim path
	role, err := extractClaimValue(claims, e.RoleClaim)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRoleClaimNotFound, err)
	}

	// Convert to string
	roleStr, ok := role.(string)
	if !ok {
		// Try to convert if it's not a string
		return fmt.Sprintf("%v", role), nil
	}

	return roleStr, nil
}

// extractClaimValue extracts a value from JWT claims using a dot-notation or array notation path.
// Examples:
//   - "role" -> claims["role"]
//   - "roles[0]" -> claims["roles"].([]any)[0]
//   - "permissions.role" -> claims["permissions"].(map[string]any)["role"]
func extractClaimValue(claims jwt.MapClaims, path string) (any, error) {
	if path == "" {
		return nil, ErrEmptyClaimPath
	}

	// Handle array notation: "roles[0]"
	if idx := strings.Index(path, "["); idx != -1 {
		return extractArrayClaim(claims, path, idx)
	}

	// Handle dot notation: "permissions.role"
	if strings.Contains(path, ".") {
		return extractNestedClaim(claims, path)
	}

	// Simple key lookup
	value, ok := claims[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClaimKeyNotFound, path)
	}

	return value, nil
}

func extractArrayClaim(claims jwt.MapClaims, path string, idx int) (any, error) {
	key := path[:idx]
	arrayPath := path[idx:]

	// Extract array index
	if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArrayNotation, path)
	}

	indexStr := strings.Trim(arrayPath, "[]")

	var index int
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArrayIndex, indexStr)
	}

	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClaimKeyNotFound, key)
	}

	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClaimNotArray, key)
	}

	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("%w: %d (length: %d)", ErrArrayIndexOutOfBounds, index, len(arr))
	}

	return arr[index], nil
}

func extractNestedClaim(claims jwt.MapClaims, path string) (any, error) {
	parts := strings.Split(path, ".")

	var current any = claims

	for i, part := range parts {
		if i == len(parts)-1 {
			return extractFinalPart(current, part, path, parts, i)
		}

		// Navigate through nested structure
		current = navigateNestedStructure(current, part, path, parts, i)
		if current == nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidClaimStructure, strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
}

func extractFinalPart(current any, part, path string, parts []string, index int) (any, error) {
	// Last part - return the value
	if m, ok := current.(map[string]any); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
		}

		return value, nil
	}

	// jwt.MapClaims is a type alias for map[string]any
	if m, ok := current.(jwt.MapClaims); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
		}

		return value, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrInvalidClaimStructure, strings.Join(parts[:index+1], "."))
}

func navigateNestedStructure(current any, part, _ string, _ []string, _ int) any {
	if m, ok := current.(map[string]any); ok {
		return m[part]
	}

	if m, ok := current.(jwt.MapClaims); ok {
		return m[part]
	}

	return nil
}
