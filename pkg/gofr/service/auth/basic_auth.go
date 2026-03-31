package auth

import (
	"context"
	"encoding/base64"
	"strings"

	"gofr.dev/pkg/gofr/service"
)

type basicAuthConfig struct {
	userName    string
	password    string
	headerValue string
}

func (*basicAuthConfig) GetHeaderKey() string {
	return service.AuthHeader
}

func (c *basicAuthConfig) GetHeaderValue(_ context.Context) (string, error) {
	return c.headerValue, nil
}

// NewBasicAuthConfig validates the provided credentials and returns a service.Options
// that injects Basic auth headers into outgoing HTTP requests.
// The password must be Base64-encoded.
func NewBasicAuthConfig(username, password string) (service.Options, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" {
		return nil, Err{Message: "username is required"}
	}

	if password == "" {
		return nil, Err{Message: "password is required"}
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(password)
	if err != nil || string(decodedPassword) == password {
		return nil, Err{Err: err, Message: "password should be base64 encoded"}
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + string(decodedPassword)))

	return NewAuthOption(&basicAuthConfig{
		userName:    username,
		password:    string(decodedPassword),
		headerValue: "Basic " + encoded,
	}), nil
}
