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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type httpService struct {
	*http.Client
	trace.Tracer
	url string
	Logger
	Metrics
}

type HTTP interface {
	// HTTP is embedded as HTTP would be able to access it's clients method
	httpClient

	// HealthCheck to get the service health and report it to the current application
	HealthCheck(ctx context.Context) *Health
	getHealthResponseForEndpoint(ctx context.Context, endpoint string, timeout int) *Health
}

type httpClient interface {
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

// NewHTTPService function creates a new instance of the httpService struct, which implements the HTTP interface.
// It initializes the http.Client, url, Tracer, and Logger fields of the httpService struct with the provided values.
func NewHTTPService(serviceAddress string, logger Logger, metrics Metrics, options ...Options) HTTP {
	h := &httpService{
		// using default HTTP client to do HTTP communication
		Client:  &http.Client{},
		url:     serviceAddress,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logger,
		Metrics: metrics,
	}

	var svc HTTP
	svc = h

	// if options are given, then add them to the httpService struct
	for _, o := range options {
		svc = o.AddOption(svc)
	}

	return svc
}

func (h *httpService) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return h.GetWithHeaders(ctx, path, queryParams, nil)
}

func (h *httpService) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
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

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// encode the query parameters on the request
	encodeQueryParameters(req, queryParams)

	// inject the TraceParent header manually in the request headers
	otel.GetTextMapPropagator().Inject(spanContext, propagation.HeaderCarrier(req.Header))

	log := &Log{
		Timestamp:     time.Now(),
		CorrelationID: trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
		HTTPMethod:    method,
		URI:           uri,
	}

	requestStart := time.Now()

	resp, err := h.Do(req)

	respTime := time.Since(requestStart)

	log.ResponseTime = respTime.Milliseconds()

	if err != nil {
		log.ResponseCode = http.StatusInternalServerError
		h.Log(&ErrorLog{Log: log, ErrorMessage: err.Error()})

		h.updateMetrics(ctx, method, respTime.Seconds(), http.StatusInternalServerError)

		return resp, err
	}

	h.updateMetrics(ctx, method, respTime.Seconds(), resp.StatusCode)
	log.ResponseCode = resp.StatusCode

	h.Log(log)

	return resp, nil
}

func (h *httpService) updateMetrics(ctx context.Context, method string, timeTaken float64, statusCode int) {
	if h.Metrics != nil {
		h.RecordHistogram(ctx, "app_http_service_response", timeTaken, "path", h.url, "method", method,
			"status", fmt.Sprintf("%v", statusCode))
	}
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
