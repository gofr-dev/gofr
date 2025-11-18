package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

type BasicAuthConfig struct {
	UserName string
	Password string
}

func (c *BasicAuthConfig) AddOption(h HTTP) HTTP {
	return &authProvider{auth: c.addAuthorizationHeader, HTTP: h}
}

// Validate implements the Validator interface for BasicAuthConfig.
// Returns an error if username or password is empty.
// Note: The password in BasicAuthConfig is already decoded (from base64).
func (c *BasicAuthConfig) Validate() error {
	username := strings.TrimSpace(c.UserName)
	password := strings.TrimSpace(c.Password)

	if username == "" {
		return AuthErr{Message: "username is required"}
	}

	if password == "" {
		return AuthErr{Message: "password is required"}
	}

	return nil
}

// FeatureName implements the Validator interface.
func (*BasicAuthConfig) FeatureName() string {
	return "Basic Authentication"
}

func NewBasicAuthConfig(username, password string) (Options, error) {
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

	config := &BasicAuthConfig{username, string(decodedPassword)}

	// Validate during creation as well for immediate feedback
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *BasicAuthConfig) addAuthorizationHeader(_ context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if value, exists := headers[AuthHeader]; exists {
		return headers, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", value, AuthHeader)}
	}

	encodedAuth := base64.StdEncoding.EncodeToString([]byte(c.UserName + ":" + c.Password))
	headers[AuthHeader] = "basic " + encodedAuth

	return headers, nil
}
