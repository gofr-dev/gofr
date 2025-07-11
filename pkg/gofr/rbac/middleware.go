package rbac

import (
	"errors"
	"net/http"

	"gofr.dev/pkg/gofr"
)

func Middleware(config *Config) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, err := RoleExtractor(config)
			if err != nil {
				http.Error(w, "Unauthorized: Missing or invalid role", http.StatusUnauthorized)
			}

			if !isPathAllowed(roles, r.URL.Path, config) {
				http.Error(w, "Forbidden: Access denied", http.StatusForbidden)
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func RequireRole(allowedRole string, handlerFunc gofr.Handler) gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var rolesWithPermission map[string][]string
		rolesWithPermission[allowedRole] = append(rolesWithPermission[allowedRole], ctx.Request.HostName())
		configs := &Config{
			RoleWithPermissions: rolesWithPermission,
		}
		roles, err := RoleExtractor(configs)
		if err != nil {
			return nil, err
		}

		for _, r := range roles {
			if r == allowedRole {
				return handlerFunc(ctx)
			}
		}

		return nil, errors.New("Forbidden: Access denied")
	}
}
