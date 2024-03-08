package service

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type SigningMethod interface {
	Verify(signingString string, sig []byte, key interface{}) error // Returns nil if signature is valid
	Sign(signingString string, key interface{}) ([]byte, error)     // Returns signature or error
	Alg() string                                                    // returns the alg identifier for this method (example: 'HS256')
}

type OAuthConfig struct {
	SigningMethod SigningMethod
	Claims        map[string]interface{}
	SecretKey     string
	Validity      time.Duration
}

func (h *OAuthConfig) addOption(svc HTTP) HTTP {
	return &oAuth{
		SigningMethod: h.SigningMethod,
		Claims:        h.Claims,
		SecretKey:     h.SecretKey,
		Validity:      h.Validity,
		HTTP:          svc,
	}
}

type oAuth struct {
	SigningMethod SigningMethod
	Claims        map[string]interface{}
	SecretKey     string
	Validity      time.Duration
	HTTP
}

func (o *oAuth) createToken() (string, error) {
	issueTime := time.Now()

	o.Claims["iss"] = issueTime
	o.Claims["exp"] = issueTime.Add(o.Validity).Unix() // Expiration time

	var claimMap jwt.MapClaims

	claimMap = o.Claims

	claims := jwt.NewWithClaims(o.SigningMethod, claimMap)

	tokenString, err := claims.SignedString([]byte(o.SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (o *oAuth) doRequest(ctx context.Context, method, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	var err error

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["Authorization"], err = o.createToken()
	if err != nil {
		return nil, err
	}

	switch method {
	case http.MethodGet:
		return o.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	case http.MethodPost:
		return o.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPatch:
		return o.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPut:
		return o.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodDelete:
		return o.HTTP.DeleteWithHeaders(ctx, path, body, headers)
	}

	return nil, nil
}

func (o *oAuth) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return o.doRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (o *oAuth) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodGet, path, queryParams, nil, nil)
}

// Post is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPost, path, queryParams, body, nil)
}

// Patch is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPatch, path, queryParams, body, nil)
}

// Put is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.doRequest(ctx, http.MethodPut, path, queryParams, body, nil)
}

// Delete is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) Delete(ctx context.Context, path string, body []byte) (
	*http.Response, error) {
	return o.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}
