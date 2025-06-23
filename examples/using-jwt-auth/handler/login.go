package handler

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr"
)

func LoginHandler(secret string) gofr.Handler {
	return func(ctx *gofr.Context) (interface{}, error) {
		username := ctx.Param("username")
		if username == "" {
			ctx.Error(http.StatusBadRequest, "Username required")
			return nil, nil
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": username,
			"exp":      time.Now().Add(time.Hour).Unix(),
		})

		tokenStr, err := token.SignedString([]byte(secret))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "Token generation failed")
			return nil, nil
		}

		return map[string]string{"token": tokenStr}, nil
	}
}
