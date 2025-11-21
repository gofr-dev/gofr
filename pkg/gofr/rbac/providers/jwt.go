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
	// ErrJWTNotEnabled is returned when JWT role extraction is attempted but OAuth is not enabled
	ErrJWTNotEnabled = errors.New("JWT/OAuth middleware not enabled")

	// ErrRoleClaimNotFound is returned when the specified role claim is not found in JWT
	ErrRoleClaimNotFound = errors.New("role claim not found in JWT")
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
		return "", fmt.Errorf("%w: %v", ErrRoleClaimNotFound, err)
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
//   - "roles[0]" -> claims["roles"].([]interface{})[0]
//   - "permissions.role" -> claims["permissions"].(map[string]interface{})["role"]
func extractClaimValue(claims jwt.MapClaims, path string) (interface{}, error) {
	if path == "" {
		return nil, errors.New("empty claim path")
	}

	// Handle array notation: "roles[0]"
	if idx := strings.Index(path, "["); idx != -1 {
		key := path[:idx]
		arrayPath := path[idx:]

		// Extract array index
		if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
			return nil, fmt.Errorf("invalid array notation: %s", path)
		}

		indexStr := strings.Trim(arrayPath, "[]")
		var index int
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
			return nil, fmt.Errorf("invalid array index: %s", indexStr)
		}

		value, ok := claims[key]
		if !ok {
			return nil, fmt.Errorf("claim key not found: %s", key)
		}

		arr, ok := value.([]interface{})
		if !ok {
			return nil, fmt.Errorf("claim value is not an array: %s", key)
		}

		if index < 0 || index >= len(arr) {
			return nil, fmt.Errorf("array index out of bounds: %d (length: %d)", index, len(arr))
		}

		return arr[index], nil
	}

	// Handle dot notation: "permissions.role"
	if strings.Contains(path, ".") {
		parts := strings.Split(path, ".")
		var current interface{} = claims

		for i, part := range parts {
			if i == len(parts)-1 {
				// Last part - return the value
				if m, ok := current.(map[string]interface{}); ok {
					value, exists := m[part]
					if !exists {
						return nil, fmt.Errorf("claim path not found: %s", path)
					}
					return value, nil
				}

				// jwt.MapClaims is a type alias for map[string]interface{}
				if m, ok := current.(jwt.MapClaims); ok {
					value, exists := m[part]
					if !exists {
						return nil, fmt.Errorf("claim path not found: %s", path)
					}
					return value, nil
				}

				return nil, fmt.Errorf("invalid claim structure at: %s", strings.Join(parts[:i+1], "."))
			}

			// Navigate through nested structure
			if m, ok := current.(map[string]interface{}); ok {
				current = m[part]
			} else if m, ok := current.(jwt.MapClaims); ok {
				current = m[part]
			} else {
				return nil, fmt.Errorf("invalid claim structure at: %s", strings.Join(parts[:i+1], "."))
			}
		}
	}

	// Simple key lookup
	value, ok := claims[path]
	if !ok {
		return nil, fmt.Errorf("claim key not found: %s", path)
	}

	return value, nil
}

