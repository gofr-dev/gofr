package auth

import (
	"context"
	"encoding/base64"
	"strings"

	"gofr.dev/pkg/gofr/service"
)

type basicAuthConfig struct {
	userName string
	password string
}

// GetHeaderKey returns the Authorization header key.
func (c *basicAuthConfig) GetHeaderKey() string {
	return AuthHeader
}

// GetHeaderValue returns the Base64-encoded Basic auth value.
func (c *basicAuthConfig) GetHeaderValue(_ context.Context) (string, error) {
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(c.userName + ":" + c.password))
	return "Basic " + encodedAuth, nil
}

// NewBasicAuthConfig validates the provided credentials and returns a service.Options
// that injects Basic auth headers into outgoing HTTP requests.
// The password must be Base64-encoded.
func NewBasicAuthConfig(username, password string) (service.Options, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" {
		return nil, AuthErr{Message: "username is required"}
	}

	if password == "" {
		return nil, AuthErr{Message: "password is required"}
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(password)
	if err != nil || string(decodedPassword) == password {
		return nil, AuthErr{Err: err, Message: "password should be base64 encoded"}
	}

	return NewAuthOption(&basicAuthConfig{userName: username, password: string(decodedPassword)}), nil
}
