package service

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
)

// Get used for making HTTP GET requests to a specified API endpoint with optional query parameters.
// It can utilize caching if available or directly perform the HTTP request, returning a Response object
// containing the response data and status code or an error if the request fails.
func (h *httpService) Get(ctx context.Context, api string, params map[string]interface{}) (*Response, error) {
	if h.cache != nil {
		return h.cache.Get(ctx, api, params)
	}

	return h.call(ctx, http.MethodGet, api, params, nil, nil)
}

// Post used for making HTTP POST requests to a specified API endpoint with optional query parameters.
func (h *httpService) Post(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error) {
	return h.call(ctx, http.MethodPost, api, params, body, nil)
}

// Put used for making HTTP PUT requests to a specified API endpoint with optional query parameters.
func (h *httpService) Put(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error) {
	return h.call(ctx, http.MethodPut, api, params, body, nil)
}

// Patch used for making HTTP PATCH requests to a specified API endpoint with optional query parameters.
func (h *httpService) Patch(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error) {
	return h.call(ctx, http.MethodPatch, api, params, body, nil)
}

// Delete used for making HTTP DELETE requests to a specified API endpoint with optional query parameters.
func (h *httpService) Delete(ctx context.Context, api string, body []byte) (*Response, error) {
	return h.call(ctx, http.MethodDelete, api, nil, body, nil)
}

// GetWithHeaders used for making HTTP GET requests to a specified API endpoint with optional query parameters and headers
// It can utilize caching if available or directly perform the HTTP request, returning a Response object
// containing the response data and status code or an error if the request fails.
func (h *httpService) GetWithHeaders(ctx context.Context, api string, params map[string]interface{},
	headers map[string]string) (*Response, error) {
	if h.cache != nil {
		return h.cache.GetWithHeaders(ctx, api, params, headers)
	}

	return h.call(ctx, "GET", api, params, nil, headers)
}

// PostWithHeaders used for making HTTP POST requests to a specified API endpoint with optional query parameters and headers
func (h *httpService) PostWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte,
	headers map[string]string) (*Response, error) {
	return h.call(ctx, http.MethodPost, api, params, body, headers)
}

// PutWithHeaders used for making HTTP PUT requests to a specified API endpoint with optional query parameters and headers
func (h *httpService) PutWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte,
	headers map[string]string) (*Response, error) {
	return h.call(ctx, http.MethodPut, api, params, body, headers)
}

// PatchWithHeaders used for making HTTP PATCH requests to a specified API endpoint with optional query parameters and headers
func (h *httpService) PatchWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte,
	headers map[string]string) (*Response, error) {
	return h.call(ctx, http.MethodPatch, api, params, body, headers)
}

// DeleteWithHeaders used for making HTTP DELETE requests to a specified API endpoint with optional query parameters and headers
func (h *httpService) DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*Response, error) {
	return h.call(ctx, http.MethodDelete, api, nil, body, headers)
}

// Bind takes Response and binds it to i based on content-type.
func (h *httpService) Bind(resp []byte, i interface{}) error {
	var err error

	h.mu.Lock()
	contentType := h.contentType
	h.mu.Unlock()

	switch contentType {
	case XML:
		err = xml.NewDecoder(bytes.NewBuffer(resp)).Decode(&i)
	case TEXT:
		v, ok := i.(*string)
		if ok {
			*v = string(resp)
		}
	case HTML, JSON:
		err = json.NewDecoder(bytes.NewBuffer(resp)).Decode(&i)
	}

	return err
}

// BindStrict deserialize HTTP response data into a Go interface while enforcing
// strict binding rules based on the content type of the response. It supports various
// content types like XML, JSON, HTML, and plain text, and it disallows unknown fields
// when deserializing JSON data. If successful, it returns nil; otherwise, it returns an error.
func (h *httpService) BindStrict(resp []byte, i interface{}) error {
	var err error

	h.mu.Lock()
	contentType := h.contentType
	h.mu.Unlock()

	switch contentType {
	case XML:
		err = xml.NewDecoder(bytes.NewBuffer(resp)).Decode(&i)
	case TEXT:
		v, ok := i.(*string)
		if ok {
			*v = string(resp)
		}
	case HTML, JSON:
		dec := json.NewDecoder(bytes.NewBuffer(resp))
		dec.DisallowUnknownFields()
		err = dec.Decode(&i)
	}

	return err
}
