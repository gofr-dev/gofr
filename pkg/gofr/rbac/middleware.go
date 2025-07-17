package rbac

import (
	"context"
	"errors"
	"net/http"

	"gofr.dev/pkg/gofr"
)

/*
roles with routes allowed- json file
extract the file and store in rbac configs
role given for the API- remove default case
*/

type authMethod int

const userRole authMethod = 4

func Middleware(config *Config, args ...any) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, err := config.RoleExtractorFunc(r, args)
			if err != nil {
				http.Error(w, "Unauthorized: Missing or invalid role", http.StatusUnauthorized)

				return
			}

			if !isPathAllowed(role, r.URL.Path, config) {
				http.Error(w, "Forbidden: Access denied", http.StatusForbidden)

				return
			}

			ctx := context.WithValue(r.Context(), userRole, role)

			handler.ServeHTTP(w, r.Clone(ctx))
		})
	}
}

func RequireRole(allowedRole string, handlerFunc gofr.Handler) gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		authinfo := ctx.GetAuthInfo()

		if authinfo.GetRole() == allowedRole {
			return handlerFunc(ctx)
		}

		return nil, errors.New("forbidden: access denied")
	}
}
