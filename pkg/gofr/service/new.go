package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Metric label names used for the app_http_service_response histogram.
// Defined as constants so each emit-site uses the same spelling.
const (
	metricLabelPath    = "path"
	metricLabelMethod  = "method"
	metricLabelStatus  = "status"
	metricLabelService = "service"
)

// methodQuery is the HTTP QUERY method (RFC 10008). It mirrors the exported
// gofrHTTP.MethodQuery / gofr.MethodQuery constant and is deliberately a second
// copy: importing gofr/http here for one string would pull gorilla/mux,
// html/template and archive/zip into the outbound HTTP client's dependency
// graph, which is nine packages the client has no other use for. (The import
// itself compiles -- there is no cycle -- so the reason is the dependency
// weight, not a build constraint.) Go's net/http has no http.MethodQuery yet.
// TestMethodQueryMatchesInboundConstant pins this string to the inbound one.
const methodQuery = "QUERY"

// redactedUserInfo replaces the credentials of a service address wherever the address is logged,
// used as a metric label or used as a span name.
const redactedUserInfo = "REDACTED"

type httpService struct {
	*http.Client
	trace.Tracer
	url string
	// redactedURL is url with its credentials replaced, set only when url carries any. It is
	// what logs, the metric label and span names show.
	redactedURL string
	name        string
	Logger
	Metrics

	// healthEndpoint is the custom endpoint for health checks (shared across options)
	healthEndpoint string
	// healthTimeout is the timeout in seconds for health check requests
	healthTimeout int
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
	Get(ctx context.Context, api string, queryParams map[string]any) (*http.Response, error)
	// GetWithHeaders performs an HTTP GET request with custom headers.
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
		headers map[string]string) (*http.Response, error)

	// Post performs an HTTP POST request.
	Post(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error)
	// PostWithHeaders performs an HTTP POST request with custom headers.
	PostWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
		headers map[string]string) (*http.Response, error)

	// Put performs an HTTP PUT request.
	Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (*http.Response, error)
	// PutWithHeaders performs an HTTP PUT request with custom headers.
	PutWithHeaders(ctx context.Context, api string, queryParams map[string]any, body []byte,
		headers map[string]string) (*http.Response, error)

	// Patch performs an HTTP PATCH request.
	Patch(ctx context.Context, api string, queryParams map[string]any, body []byte) (*http.Response, error)
	// PatchWithHeaders performs an HTTP PATCH request with custom headers.
	PatchWithHeaders(ctx context.Context, api string, queryParams map[string]any, body []byte,
		headers map[string]string) (*http.Response, error)

	// Delete performs an HTTP DELETE request.
	Delete(ctx context.Context, api string, body []byte) (*http.Response, error)
	// DeleteWithHeaders performs an HTTP DELETE request with custom headers.
	DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*http.Response, error)

	// Query performs an HTTP QUERY request (RFC 10008). QUERY is a safe, idempotent
	// method that carries the query in the request body.
	Query(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error)
	// QueryWithHeaders performs an HTTP QUERY request with custom headers.
	QueryWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
		headers map[string]string) (*http.Response, error)
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

	if redacted := redactAddress(serviceAddress); redacted != serviceAddress {
		h.redactedURL = redacted
	}

	var svc HTTP

	svc = h

	// if options are given, then add them to the httpService struct.
	// Options that implement Observable receive the logger and metrics
	// before AddOption so they can wire up out-of-band observability
	// (e.g. background goroutines) without taking these from the caller.
	for _, o := range options {
		if obs, ok := o.(Observable); ok {
			obs.SetLogger(logger)
			obs.SetMetrics(metrics)
		}

		svc = o.AddOption(svc)
	}

	return svc
}

func (h *httpService) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return h.GetWithHeaders(ctx, path, queryParams, nil)
}

func (h *httpService) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

func (h *httpService) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return h.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

func (h *httpService) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error) {
	return h.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

func (h *httpService) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return h.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

func (h *httpService) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return h.DeleteWithHeaders(ctx, path, body, nil)
}

func (h *httpService) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (h *httpService) Query(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return h.QueryWithHeaders(ctx, path, queryParams, body, nil)
}

func (h *httpService) QueryWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return h.createAndSendRequest(ctx, methodQuery, path, queryParams, body, headers)
}

func (h *httpService) createAndSendRequest(ctx context.Context, method string, path string,
	queryParams map[string]any, body []byte, headers map[string]string) (*http.Response, error) {
	uri := h.url + "/" + path
	uri = strings.TrimRight(uri, "/")

	// The request is sent to uri; logURI is the same address without its credentials.
	logURI := uri
	if h.redactedURL != "" {
		logURI = strings.TrimRight(h.redactedURL+"/"+path, "/")
	}

	ctx, span := h.Tracer.Start(ctx, logURI)
	defer span.End()

	// Attach client-side trace handling for HTTP request, but only when
	// the span is actually recording. otelhttptrace.NewClientTrace builds
	// a httptrace.ClientTrace with many event callbacks (DNS, connect,
	// TLS, etc.) — under a noop tracer (post PR-1, the default when no
	// exporter is configured) those events are discarded, so the
	// allocation is pure waste.
	clientTraceCtx := ctx
	if span.IsRecording() {
		clientTraceCtx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
	}

	// Create the HTTP request with the tracing context.
	req, err := http.NewRequestWithContext(clientTraceCtx, method, uri, bytes.NewBuffer(body))
	if err != nil {
		return nil, redactURLError(err)
	}

	var isContentTypeSet bool

	for k, v := range headers {
		if strings.EqualFold(k, "content-type") {
			isContentTypeSet = true
		}

		req.Header.Set(k, v)
	}

	if !isContentTypeSet {
		req.Header.Set("Content-Type", "application/json")
	}

	// Inject tracing information into the request headers.
	otel.GetTextMapPropagator().Inject(clientTraceCtx, propagation.HeaderCarrier(req.Header))

	// encode the query parameters on the request.
	encodeQueryParameters(req, queryParams)

	log := &Log{
		Timestamp:     time.Now(),
		CorrelationID: trace.SpanFromContext(clientTraceCtx).SpanContext().TraceID().String(),
		HTTPMethod:    method,
		URI:           logURI,
	}

	requestStart := time.Now()

	resp, err := h.Do(req)

	respTime := time.Since(requestStart)

	log.ResponseTime = respTime.Microseconds()

	if err != nil {
		err = redactURLError(err)

		log.ResponseCode = http.StatusServiceUnavailable
		h.Log(&ErrorLog{Log: log, ErrorMessage: err.Error()})

		h.updateMetrics(clientTraceCtx, method, respTime.Seconds(), http.StatusServiceUnavailable)

		return resp, err
	}

	h.updateMetrics(clientTraceCtx, method, respTime.Seconds(), resp.StatusCode)
	log.ResponseCode = resp.StatusCode

	h.Log(log)

	return resp, nil
}

func (h *httpService) updateMetrics(ctx context.Context, method string, timeTaken float64, statusCode int) {
	if h.Metrics != nil {
		labels := []string{
			metricLabelPath, h.logAddress(),
			metricLabelMethod, method,
			metricLabelStatus, fmt.Sprintf("%v", statusCode),
		}

		if h.name != "" {
			labels = append(labels, metricLabelService, h.name)
		}

		h.RecordHistogram(ctx, "app_http_service_response", timeTaken, labels...)
	}
}

func encodeQueryParameters(req *http.Request, queryParams map[string]any) {
	q := req.URL.Query()

	for k, v := range queryParams {
		switch vt := v.(type) {
		case []string:
			for _, val := range vt {
				q.Add(k, val)
			}
		default:
			q.Set(k, fmt.Sprintf("%v", v))
		}
	}

	req.URL.RawQuery = q.Encode()
}

type attributesOption map[string]string

func (a attributesOption) AddOption(h HTTP) HTTP {
	if svc := extractHTTPService(h); svc != nil {
		if name, ok := a["name"]; ok {
			svc.name = name
		}
	}

	return h
}

// WithAttributes returns an Option that sets the attributes of the HTTP service.
func WithAttributes(attributes map[string]string) Options {
	return attributesOption(attributes)
}

// logAddress returns the service address as logs, the metric label and span names show it.
func (h *httpService) logAddress() string {
	if h.redactedURL != "" {
		return h.redactedURL
	}

	return h.url
}

// redactURLError replaces the address inside a *url.Error, which net/http returns for a failed
// request and net/url for an address it cannot parse. net/http masks only a password
// (user:***@), so a token used as the user name would otherwise reach the logs and the caller.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = redactAddress(urlErr.URL)
	}

	return err
}

// redactAddress returns address with its userinfo (https://user:pass@host) replaced, so it can be
// logged, used as a metric label or used as a span name. An address without userinfo is returned
// unchanged.
func redactAddress(address string) string {
	if !strings.Contains(address, "@") {
		return address
	}

	u, err := url.Parse(address)
	if err != nil {
		// A bad escape in a password fails the parse; drop everything up to the last '@'.
		return redactedUserInfo + address[strings.LastIndex(address, "@"):]
	}

	if u.User == nil {
		return address
	}

	u.User = url.User(redactedUserInfo)

	return u.String()
}
