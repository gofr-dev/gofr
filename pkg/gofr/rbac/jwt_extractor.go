package rbac

import (
	"net/http"

	"gofr.dev/pkg/gofr/rbac/providers"
)

// NewJWTRoleExtractor creates a new JWT role extractor.
// This function delegates to the providers package to avoid import cycles.
func NewJWTRoleExtractor(roleClaim string) JWTRoleExtractorProvider {
	return providers.NewJWTRoleExtractor(roleClaim)
}

// JWTRoleExtractorProvider is an interface for JWT-based role extraction.
type JWTRoleExtractorProvider interface {
	ExtractRole(req *http.Request, args ...any) (string, error)
}

