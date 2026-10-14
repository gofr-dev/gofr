package service

import (
	"context"
	"encoding/base64"
	"strings"
)

const (
	msgUsernameRequired  = "username is required"
	msgPasswordRequired  = "password is required"
	msgPasswordNotBase64 = "password should be base64 encoded"
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
		return nil, AuthErr{Message: msgUsernameRequired}
	}

	if password == "" {
		return nil, AuthErr{Message: msgPasswordRequired}
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(password)
	if err != nil || string(decodedPassword) == password {
		return nil, AuthErr{Err: err, Message: msgPasswordNotBase64}
	}

	return &BasicAuthConfig{username, string(decodedPassword)}, nil
}

func (c *BasicAuthConfig) addAuthorizationHeader(_ context.Context, headers map[string]string) (map[string]string, error) {
	headers = cloneHeaders(headers)

	if _, exists := headers[AuthHeader]; exists {
		return headers, AuthErr{Message: headerExistsMessage(AuthHeader)}
	}

	encodedAuth := base64.StdEncoding.EncodeToString([]byte(c.UserName + ":" + c.Password))
	headers[AuthHeader] = "Basic " + encodedAuth

	return headers, nil
}
