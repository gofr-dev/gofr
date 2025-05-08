package service

import (
	"context"
	b64 "encoding/base64"
	"net/http"
)

type BasicAuthConfig struct {
	UserName string
	Password string
}

func (a *BasicAuthConfig) AddOption(h HTTP) HTTP {
	return &basicAuthProvider{
		userName: a.UserName,
		password: a.Password,
		HTTP:     h,
	}
}

type basicAuthProvider struct {
	userName string
	password string

	HTTP
}

func (bap *basicAuthProvider) addAuthorizationHeader(headers map[string]string) error {
	decodedPassword, err := b64.StdEncoding.DecodeString(bap.password)
	if err != nil {
		return err
	}

	encodedAuth := b64.StdEncoding.EncodeToString([]byte(bap.userName + ":" + string(decodedPassword)))

	headers["Authorization"] = "basic " + encodedAuth

	return nil
}

func (bap *basicAuthProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return bap.GetWithHeaders(ctx, path, queryParams, nil)
}

func (bap *basicAuthProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	err := bap.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return bap.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (bap *basicAuthProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return bap.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (bap *basicAuthProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	err := bap.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return bap.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (bap *basicAuthProvider) Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (*http.Response, error) {
	return bap.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (bap *basicAuthProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	err := bap.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return bap.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (bap *basicAuthProvider) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return bap.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (bap *basicAuthProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	err := bap.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return bap.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (bap *basicAuthProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return bap.DeleteWithHeaders(ctx, path, body, nil)
}

func (bap *basicAuthProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	err := bap.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return bap.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func (bap *basicAuthProvider) populateHeaders(headers map[string]string) error {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := bap.addAuthorizationHeader(headers)
	if err != nil {
		return err
	}

	return nil
}
