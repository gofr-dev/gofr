package service

import (
	"bytes"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/http/httptrace"
	"strings"

	"gofr.dev/pkg/gofr"
)

type httpService struct {
	*http.Client
	trace.Tracer
	url string
}

type HTTP interface {
	Get(ctx *gofr.Context, api string, queryParams map[string]interface{}) (*http.Response, error)
	GetWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error)

	Post(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	PostWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error)

	Put(ctx *gofr.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error)
	PutWithHeaders(ctx *gofr.Context, api string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error)

	Delete(ctx *gofr.Context, api string, body []byte) (*http.Response, error)
	DeleteWithHeaders(ctx *gofr.Context, api string, body []byte, headers map[string]string) (*http.Response, error)
}

func NewHTTPService(serviceAddress string) HTTP {
	return &httpService{
		Client: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		url:    serviceAddress,
		Tracer: otel.Tracer("gofr-http-client"),
	}
}

func (h *httpService) Get(ctx *gofr.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, nil)
}

func (h *httpService) GetWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

func (h *httpService) Post(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, nil)
}

func (h *httpService) PostWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

func (h *httpService) Patch(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PatchWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

func (h *httpService) Put(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return h.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PutWithHeaders(ctx *gofr.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

func (h *httpService) Delete(ctx *gofr.Context, path string, body []byte) (*http.Response, error) {
	return h.DeleteWithHeaders(ctx, path, body, nil)
}

func (h *httpService) DeleteWithHeaders(ctx *gofr.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (h *httpService) createAndSendRequest(ctx *gofr.Context, method string, path string,
	queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	uri := h.url + "/" + path
	uri = strings.TrimRight(uri, "/")

	spanContext, span := h.Tracer.Start(ctx, uri)
	defer span.End()

	spanContext = httptrace.WithClientTrace(spanContext, otelhttptrace.NewClientTrace(ctx))
	req, _ := http.NewRequestWithContext(spanContext, method, uri, bytes.NewBuffer(body))

	// set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	encodeQueryParameters(req, queryParams)

	resp, err := h.Do(req)

	ctx.Logger.Log(HTTPCallLog{
		MessageId:    "test",
		ResponseCode: resp.StatusCode,
		ResponseTime: 0,
		Method:       method,
		URI:          uri,
	})

	return resp, err
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
