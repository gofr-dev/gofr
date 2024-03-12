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

func NewRequest(r *http.Request) *Request {
	return &Request{
		req:        r,
		pathParams: mux.Vars(r),
	}
}

func (r *Request) Param(key string) string {
	return r.req.URL.Query().Get(key)
}

func (r *Request) Context() context.Context {
	return r.req.Context()
}

func (r *Request) PathParam(key string) string {
	return r.pathParams[key]
}

func (r *Request) Bind(i interface{}) error {
	body, err := r.body()
	if err != nil {
		return err
	}

	return json.Unmarshal(body, &i)
}

func (r *Request) GetClaims() map[string]interface{} {
	claims, ok := r.Context().Value("JWTClaims").(jwt.MapClaims)
	if !ok {
		return nil
	}

	return claims
}

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
