package oauth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"gofr.dev/pkg/middleware"

	"gofr.dev/pkg/log"
)

func getJWT(logger log.Logger, r *http.Request) (JWT, error) {
	token := r.Header.Get("Authorization")

	jwtVal := strings.Fields(token)
	if token == "" || len(jwtVal) != 2 || !strings.EqualFold(jwtVal[0], "bearer") {
		logger.Error("invalid format for authorization header")
		return JWT{}, middleware.ErrInvalidRequest
	}

	// Checking if incoming token string conforms to the predefined jwt structure
	jwtParts := strings.Split(jwtVal[1], ".")

	const jwtPartsLen = 3
	if len(jwtParts) != jwtPartsLen {
		logger.Error("jwt token is not of the format hhh.ppp.sss")
		return JWT{}, middleware.ErrInvalidToken
	}

	var h header

	decodedHeader, err := base64.RawStdEncoding.DecodeString(jwtParts[0])
	if err != nil {
		logger.Error("Failed to decode jwt header: ", err)
		return JWT{}, middleware.ErrInvalidToken
	}

	err = json.Unmarshal(decodedHeader, &h)
	if err != nil {
		logger.Error("Failed to unmarshal jwt header: ", err)
		return JWT{}, middleware.ErrInvalidToken
	}

	return JWT{payload: jwtParts[1], header: h, signature: jwtParts[2], token: jwtVal[1]}, nil
}
