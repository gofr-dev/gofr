package service

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"strings"
)

type BasicAuthConfig struct {
	UserName string
	Password string
}

func (a *BasicAuthConfig) AddOption(h HTTP) HTTP {
	return &basicAuthProvider{
		userName:     a.UserName,
		password:     a.Password,
		authProvider: authProvider{h},
	}
}

type basicAuthProvider struct {
	userName string
	password string

	authProvider
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

	decodedPassword, err := b64.StdEncoding.DecodeString(password)
	if err != nil {
		return nil, AuthErr{Err: err, Message: "password should be base64 encoded"}
	}

	return &BasicAuthConfig{username, string(decodedPassword)}, nil
}

func (a *basicAuthProvider) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	if value, exists := headers[AuthHeader]; exists {
		return headers, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", value, AuthHeader)}
	}
	encodedAuth := b64.StdEncoding.EncodeToString([]byte(a.userName + ":" + a.password))
	headers[AuthHeader] = "basic " + encodedAuth
	return headers, nil
}
