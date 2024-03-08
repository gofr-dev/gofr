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

	var claimMap jwt.MapClaims = o.Claims

	claims := jwt.NewWithClaims(o.SigningMethod, claimMap)

	tokenString, err := claims.SignedString([]byte(o.SecretKey))
	if err != nil {
		return "", err
	}

	return "Bearer " + tokenString, nil
}

func (o *oAuth) addAuthorizationHeader(headers map[string]string) (map[string]string, error) {
	var err error

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["Authorization"], err = o.createToken()
	if err != nil {
		return nil, err
	}

	return headers, nil
}

func (o *oAuth) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	headers, err := o.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func (o *oAuth) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return o.GetWithHeaders(ctx, path, queryParams, nil)
}

// Post is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PostWithHeaders(ctx, path, queryParams, body, nil)
}

// Patch is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

// Put is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PutWithHeaders(ctx, path, queryParams, body, nil)
}

// Delete is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) Delete(ctx context.Context, path string, body []byte) (
	*http.Response, error) {
	return o.DeleteWithHeaders(ctx, path, body, nil)
}
