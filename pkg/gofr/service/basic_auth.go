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

	return &BasicAuthConfig{username, string(decodedPassword)}, nil
}

func (c *BasicAuthConfig) addAuthorizationHeader(_ context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if value, exists := headers[AuthHeader]; exists {
		return headers, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", value, AuthHeader)}
	}

	encodedAuth := base64.StdEncoding.EncodeToString([]byte(c.UserName + ":" + c.Password))
	headers[AuthHeader] = "Basic " + encodedAuth

	return headers, nil
}
