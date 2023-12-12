package oauth

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v4"

	"gofr.dev/pkg/middleware"

	"gofr.dev/pkg/log"
)

// LDAPOAuth handles LDAP and OAuth authentication. It grants access based on successful LDAP or OAuth validation
func LDAPOAuth(logger log.Logger, ldapOptions *middleware.LDAPOptions, options Options) func(inner http.Handler) http.Handler {
	ldap := middleware.NewLDAP(logger, ldapOptions)

	oAuth := New(logger, options)

	return func(inner http.Handler) http.Handler {
		// in case the LDAP address or JWK End point is not configured, then LDAP OAuth middleware is ignored
		if !verifyOptions(logger, ldapOptions, &options) {
			return inner
		}

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if middleware.ExemptPath(req) {
				inner.ServeHTTP(w, req)
				return
			}

			var token *jwt.Token
			var err error

			if err = ldap.Validate(logger, req); err == nil {
				inner.ServeHTTP(w, req)
				return
			} else if token, err = oAuth.Validate(logger, req); err == nil {
				jwtClaimsKey := JWTContextKey("claims")
				ctx := context.WithValue(req.Context(), jwtClaimsKey, token.Claims)
				req = req.Clone(ctx)
				inner.ServeHTTP(w, req)
				return
			}

			description, code := middleware.GetDescription(err)
			e := middleware.FetchErrResponseWithCode(code, description, err.Error())
			middleware.ErrorResponse(w, req, logger, *e)
		})
	}
}

func verifyOptions(logger log.Logger, ldapOptions *middleware.LDAPOptions, oAuthOptions *Options) bool {
	if ldapOptions == nil || ldapOptions.Addr == "" {
		logger.Warn("LDAP OAuth Middleware not enabled due to empty LDAP options/ missing LDAP Address")
		return false
	}

	if oAuthOptions == nil || oAuthOptions.JWKPath == "" {
		logger.Warn("LDAP OAuth Middleware not enabled due to empty oAuth options/ missing JWK End point.")
		return false
	}

	if len(ldapOptions.RegexToMethodGroup) == 0 {
		logger.Warn("LDAP OAuth Middleware not enabled due to no mappings defined for LDAP groups")
		return false
	}

	return true
}
