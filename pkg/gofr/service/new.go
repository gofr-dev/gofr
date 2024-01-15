package service

import (
	"bytes"
	"context"
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

type Context interface {
	context.Context
	GetID() string
}

type httpService struct {
	*http.Client
	trace.Tracer
	url string
	Logger
}

type HTTP interface {
	Get(ctx Context, api string, queryParams map[string]interface{}) (*http.Response, error)
	GetWithHeaders(ctx Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error)

	Post(ctx Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	PostWithHeaders(ctx Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error)

	Put(ctx Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	PutWithHeaders(ctx Context, api string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error)

	Delete(ctx Context, api string, body []byte) (*http.Response, error)
	DeleteWithHeaders(ctx Context, api string, body []byte, headers map[string]string) (*http.Response, error)
}

func NewHTTPService(serviceAddress string, logger Logger) HTTP {
	return &httpService{
		Client: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		url:    serviceAddress,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logger,
	}
}

func (h *httpService) Get(ctx Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, nil)
}

func (h *httpService) GetWithHeaders(ctx Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

func (h *httpService) Post(ctx Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, nil)
}

func (h *httpService) PostWithHeaders(ctx Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

func (h *httpService) Patch(ctx Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PatchWithHeaders(ctx Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

func (h *httpService) Put(ctx Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PutWithHeaders(ctx Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

func (h *httpService) Delete(ctx Context, path string, body []byte) (*http.Response, error) {
	return h.DeleteWithHeaders(ctx, path, body, nil)
}

func (h *httpService) DeleteWithHeaders(ctx Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (h *httpService) createAndSendRequest(ctx Context, method string, path string,
	queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	uri := h.url + "/" + path
	uri = strings.TrimRight(uri, "/")

	spanContext, span := h.Tracer.Start(ctx, uri)
	defer span.End()

	spanContext = httptrace.WithClientTrace(spanContext, otelhttptrace.NewClientTrace(ctx))
	req, _ := http.NewRequestWithContext(spanContext, method, uri, bytes.NewBuffer(body))

	reqID := ctx.GetID()

	// set headers
	req.Header.Set("X-Correlation-Id", reqID)

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

	log.ResponseTime = time.Since(requestStart).Nanoseconds() / 1000

	if err != nil {
		log.ResponseCode = http.StatusInternalServerError
		h.Error(ErrorLog{Log: log, ErrorMessage: err.Error()})

		return resp, err
	}

	log.ResponseCode = resp.StatusCode

	h.Log(log)

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
