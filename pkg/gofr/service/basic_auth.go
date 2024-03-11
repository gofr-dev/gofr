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

func (a *BasicAuthConfig) addOption(h HTTP) HTTP {
	return &BasicAuthProvider{
		userName: a.UserName,
		password: a.Password,
		HTTP:     h,
	}
}

type BasicAuthProvider struct {
	userName string
	password string

	HTTP
}

func (ba *BasicAuthProvider) addAuthorizationHeader(headers map[string]string) error {
	decodedPassword, err := b64.StdEncoding.DecodeString(ba.password)
	if err != nil {
		return err
	}

	encodedAuth := b64.StdEncoding.EncodeToString([]byte(ba.userName + ":" + string(decodedPassword)))

	headers["Authorization"] = "basic " + encodedAuth

	return nil
}

func (ba *BasicAuthProvider) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return ba.GetWithHeaders(ctx, path, queryParams, nil)
}

func (ba *BasicAuthProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	err := ba.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return ba.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (ba *BasicAuthProvider) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return ba.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (ba *BasicAuthProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	err := ba.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return ba.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (ba *BasicAuthProvider) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return ba.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (ba *BasicAuthProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	err := ba.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return ba.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (ba *BasicAuthProvider) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return ba.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (ba *BasicAuthProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	err := ba.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return ba.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (ba *BasicAuthProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return ba.DeleteWithHeaders(ctx, path, body, nil)
}

func (ba *BasicAuthProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	err := ba.populateHeaders(headers)
	if err != nil {
		return nil, err
	}

	return ba.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func (ba *BasicAuthProvider) populateHeaders(headers map[string]string) error {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := ba.addAuthorizationHeader(headers)
	if err != nil {
		return err
	}

	return nil
}
