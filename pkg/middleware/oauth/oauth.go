/*
Package oauth provides a middleware for authenticating requests.This package provides functionality for token validation
and integration with JSON Web Key (JWK) to verify JSON Web Tokens (JWT).
*/
package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v4"

	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

type JWTContextKey string

// New is a factory function that creates and initializes an oAuth instance
func New(logger log.Logger, options Options) (oAuth *OAuth) {
	oAuth = &OAuth{options: options, cache: PublicKeyCache{
		publicKeys: PublicKeys{},
		mu:         sync.RWMutex{},
	}}

	if strings.TrimSpace(options.JWKPath) != "" {
		_ = oAuth.invalidateCache(logger)
	}

	return
}

// Auth defines an HTTP middleware for OAuth authentication.
// It allows access if the token is valid
func Auth(logger log.Logger, options Options) func(inner http.Handler) http.Handler {
	oAuth := New(logger, options)

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if middleware.ExemptPath(req) || strings.TrimSpace(oAuth.options.JWKPath) == "" {
				inner.ServeHTTP(w, req)
				return
			}

			jwtClaimsKey := JWTContextKey("claims")
			token, err := oAuth.Validate(logger, req)
			if err == nil {
				// if user is verified ,setting it in the context pool
				ctx := context.WithValue(req.Context(), jwtClaimsKey, token.Claims)
				*req = *req.Clone(ctx)
				inner.ServeHTTP(w, req)
				return
			}
			logger.Errorf("Client authentication failed for given token with Error : %v", err)

			description, code := middleware.GetDescription(err)
			e := middleware.FetchErrResponseWithCode(code, description, err.Error())

			middleware.ErrorResponse(w, req, logger, *e)
		})
	}
}

// Validate checks if the token present in header is in jwt format or not.
// If the format is correct: public key is got from endpoint and RSA to verify if the token is valid.
func (o *OAuth) Validate(logger log.Logger, r *http.Request) (*jwt.Token, error) {
	token := &jwt.Token{Valid: false}

	jwtObj, err := getJWT(logger, r)
	if err != nil {
		return token, err
	}

	// fetching public key for the specified header key id
	publicKey := o.cache.publicKeys.Get(jwtObj.header.KeyID)

	// generating RSA public key format for the saved public key
	// to validate if incoming token is not tampered
	pKey, err := publicKey.getRSAPublicKey()
	if err != nil {
		logger.Errorf("Error while getting public key: %v", err)
		return token, middleware.ErrInvalidToken
	}

	claims := jwt.MapClaims{}

	// validation of token
	token, err = jwt.ParseWithClaims(jwtObj.token, claims, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodRSA)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// provide the rsa kID
		return &pKey, nil
	})

	if err != nil {
		logger.Errorf("Failed to parse token: %v", err)
		return token, middleware.ErrInvalidToken
	}

	if !token.Valid {
		logger.Error("Invalid token")
		return token, middleware.ErrInvalidToken
	}

	return token, nil
}
