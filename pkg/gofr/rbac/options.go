package rbac

import (
	"fmt"
	"net/http"

	"gofr.dev/pkg/gofr/rbac/providers"
)

// HeaderRoleExtractor implements Options for header-based role extraction.
type HeaderRoleExtractor struct {
	// HeaderKey is the HTTP header key name (e.g., "X-User-Role")
	// Default: "X-User-Role"
	HeaderKey string
}

// AddOption configures the RBAC config with header-based role extraction.
// This follows the same pattern as service.Options for consistency.
func (h *HeaderRoleExtractor) AddOption(config RBACConfig) RBACConfig {
	if h.HeaderKey == "" {
		h.HeaderKey = "X-User-Role" // Default header key
	}

	config.SetRoleExtractorFunc(func(req *http.Request, args ...any) (string, error) {
		role := req.Header.Get(h.HeaderKey)
		if role == "" {
			return "", fmt.Errorf("role header %q not found", h.HeaderKey)
		}
		return role, nil
	})
	return config
}

// JWTExtractor implements Options for JWT-based role extraction.
type JWTExtractor struct {
	// Claim is the JWT claim path (e.g., "role", "roles[0]", "permissions.role")
	// Default: "role"
	Claim string
}

// AddOption configures the RBAC config with JWT-based role extraction.
// This follows the same pattern as service.Options for consistency.
func (j *JWTExtractor) AddOption(config RBACConfig) RBACConfig {
	if j.Claim == "" {
		j.Claim = "role" // Default claim
	}

	// Use the provider's JWT extractor
	jwtExtractor := providers.NewJWTRoleExtractor(j.Claim)
	config.SetRoleExtractorFunc(jwtExtractor.ExtractRole)
	return config
}

