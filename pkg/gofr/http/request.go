package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

// Request is an abstraction over the underlying http.Request. This abstraction is useful because it allows us
// to create applications without being aware of the transport. cmd.Request is another such abstraction.
type Request struct {
	req        *http.Request
	pathParams map[string]string
}

// NewRequest creates a new GoFr Request instance from the given http.Request.
func NewRequest(r *http.Request) *Request {
	return &Request{
		req:        r,
		pathParams: mux.Vars(r),
	}
}

// Param returns the query parameter with the given key.
func (r *Request) Param(key string) string {
	return r.req.URL.Query().Get(key)
}

// Context returns the context of the request.
func (r *Request) Context() context.Context {
	return r.req.Context()
}

// PathParam retrieves a path parameter from the request.
func (r *Request) PathParam(key string) string {
	return r.pathParams[key]
}

// Bind parses the request body and binds it to the provided interface.
func (r *Request) Bind(i interface{}) error {
	body, err := r.body()
	if err != nil {
		return err
	}

	return json.Unmarshal(body, &i)
}

// GetClaims retrieves JWT claims from the request context.
func (r *Request) GetClaims() map[string]interface{} {
	claims, ok := r.Context().Value("JWTClaims").(jwt.MapClaims)
	if !ok {
		return nil
	}

	return claims
}

// HostName retrieves the hostname from the request.
func (r *Request) HostName() string {
	proto := r.req.Header.Get("X-forwarded-proto")
	if proto == "" {
		proto = "http"
	}

	return fmt.Sprintf("%s://%s", proto, r.req.Host)
}

func (r *Request) body() ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.req.Body)
	if err != nil {
		return nil, err
	}

	r.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}
