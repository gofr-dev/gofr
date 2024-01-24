package service

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type httpService struct {
	*http.Client
	trace.Tracer
	url string
	Logger
	cache *Cache
}

type HTTP interface {
	// Get performs an HTTP GET request.
	Get(ctx context.Context, api string, queryParams map[string]interface{}) (*http.Response, error)
	// GetWithHeaders performs an HTTP GET request with custom headers.
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
		headers map[string]string) (*http.Response, error)

	// Post performs an HTTP POST request.
	Post(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	// PostWithHeaders performs an HTTP POST request with custom headers.
	PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
		headers map[string]string) (*http.Response, error)

	// Put performs an HTTP PUT request.
	Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	// PutWithHeaders performs an HTTP PUT request with custom headers.
	PutWithHeaders(ctx context.Context, api string, queryParams map[string]interface{}, body []byte,
		headers map[string]string) (*http.Response, error)

	// Patch performs an HTTP PATCH request.
	Patch(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	// PatchWithHeaders performs an HTTP PATCH request with custom headers.
	PatchWithHeaders(ctx context.Context, api string, queryParams map[string]interface{}, body []byte,
		headers map[string]string) (*http.Response, error)

	// Delete performs an HTTP DELETE request.
	Delete(ctx context.Context, api string, body []byte) (*http.Response, error)
	// DeleteWithHeaders performs an HTTP DELETE request with custom headers.
	DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*http.Response, error)
}

func NewHTTPService(serviceAddress string, logger Logger, options ...Options) HTTP {
	h := &httpService{
		Client: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		url:    serviceAddress,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logger,
	}

	// if options are given, then add them to the httpService struct
	if options != nil {
		for _, o := range options {
			o.apply(h)
		}
	}

	return h
}

func (h *httpService) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return h.GetWithHeaders(ctx, path, queryParams, nil)
}

func (h *httpService) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	if h.cache != nil {
		return h.getCachedResponse(ctx, http.MethodGet, path, queryParams, nil, headers)
	}

	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

func (h *httpService) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return h.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

func (h *httpService) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

func (h *httpService) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return h.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

func (h *httpService) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return h.DeleteWithHeaders(ctx, path, body, nil)
}

func (h *httpService) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (h *httpService) createAndSendRequest(ctx context.Context, method string, path string,
	queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	uri := h.url + "/" + path
	uri = strings.TrimRight(uri, "/")

	spanContext, span := h.Tracer.Start(ctx, uri)
	defer span.End()

	spanContext = httptrace.WithClientTrace(spanContext, otelhttptrace.NewClientTrace(ctx))

	req, err := http.NewRequestWithContext(spanContext, method, uri, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	reqID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	encodeQueryParameters(req, queryParams)

	log := Log{
		Timestamp:     time.Now(),
		CorrelationID: reqID,
		HTTPMethod:    method,
		URI:           uri,
	}

	requestStart := time.Now()

	resp, err := h.Do(req)

	log.ResponseTime = time.Since(requestStart).Microseconds()

	if err != nil {
		log.ResponseCode = http.StatusInternalServerError
		h.Log(ErrorLog{Log: log, ErrorMessage: err.Error()})

		return resp, err
	}

	log.ResponseCode = resp.StatusCode

	h.Log(log)

	return resp, nil
}

func (h *httpService) getCachedResponse(ctx context.Context,
	method string, path string,
	queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
		key  = h.url + method + path
	)

	for _, param := range queryParams {
		key += fmt.Sprintf("%v_", param)
	}

	for _, header := range headers {
		key += fmt.Sprintf("%v_", header)
	}

	// TODO - make this key fix sized // example - hashing

	// get the response stored in the cacher
	r, _ := h.cache.Get(ctx, key)

	if string(r) == "" {
		resp, err = h.createAndSendRequest(ctx, method, path, queryParams, body, headers)
	} else {
		return deserializeResponse(r)
	}

	// checking for any error while calling http service
	if err != nil {
		return nil, err
	}

	// serialize the response and add it to cache
	val, _ := serializeResponse(resp)

	// not required to check any errors
	_ = h.cache.Set(ctx, key, val, h.cache.ttl)

	return resp, nil
}

func encodeQueryParameters(req *http.Request, queryParams map[string]interface{}) {
	q := req.URL.Query()

	for k, v := range queryParams {
		switch vt := v.(type) {
		case []string:
			for _, val := range vt {
				q.Set(k, val)
			}
		default:
			q.Set(k, fmt.Sprintf("%v", v))
		}
	}

	req.URL.RawQuery = q.Encode()
}

// serializeResponse serializes an *http.Response to a []byte using gob.
func serializeResponse(response *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(response)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// deserializeResponse deserializes a []byte to an *http.Response using gob.
func deserializeResponse(data []byte) (*http.Response, error) {
	var response http.Response
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
