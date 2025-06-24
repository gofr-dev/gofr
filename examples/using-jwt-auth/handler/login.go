package handler

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr"
)

// LoginHandler returns a handler that generates a JWT token for a given username.
// This handler expects a query parameter: `username`.
//
// Example request:
//   GET /login?username=foo
//
// Example response:
//   {
//     "token": "<JWT token here>"
//   }
func LoginHandler(secret string) gofr.Handler {
	return func(ctx *gofr.Context) (interface{}, error) {
		// Extract username from query parameters
		username := ctx.Param("username")
		if username == "" {
			// Respond with HTTP 400 if username is missing
			ctx.Error(http.StatusBadRequest, "Username is required to generate a token")
			return nil, nil
		}

		// Create a new JWT token with claims
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": username,
			"exp":      time.Now().Add(time.Hour).Unix(), // Token valid for 1 hour
		})

		// Sign the token using the secret key
		tokenStr, err := token.SignedString([]byte(secret))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "Failed to generate JWT token")
			return nil, nil
		}

		// Return the token as JSON
		return map[string]string{"token": tokenStr}, nil
	}
}
