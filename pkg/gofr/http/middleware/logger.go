package middleware

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

var errHijackNotSupported = errors.New("response writer does not support hijacking")

// StatusResponseWriter Defines own Response Writer to be used for logging of status - as http.ResponseWriter does not let us read status.
type StatusResponseWriter struct {
	http.ResponseWriter
	status int
	// wroteHeader keeps a flag to keep a check that the framework do not attempt to write the header again. This was previously causing
	// `superfluous response.WriteHeader call`. This is particularly helpful in scenarios where the developer has already written header
	// in any custom middlewares.
	wroteHeader bool
}

func (w *StatusResponseWriter) WriteHeader(status int) {
	if w.wroteHeader { // Prevent duplicate calls
		return
	}

	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

// Write implements http.ResponseWriter. When a handler calls Write without
// first calling WriteHeader, net/http implicitly sends StatusOK on the
// wire — record that explicitly here so logs / metrics / tracing see 200
// instead of 0 for the common "just write the body" pattern.
func (w *StatusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}

	return w.ResponseWriter.Write(b)
}

// Hijack implements the http.Hijacker interface. So that we are able to upgrade to a websocket
// connection that requires the responseWriter implementation to implement this method.
func (w *StatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}

	return nil, nil, fmt.Errorf("%w: cannot hijack connection", errHijackNotSupported)
}

// RequestLog represents a log entry for HTTP requests.
type RequestLog struct {
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	ResponseTime int64  `json:"response_time,omitempty"`
	Method       string `json:"method,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	IP           string `json:"ip,omitempty"`
	URI          string `json:"uri,omitempty"`
	Response     int    `json:"response,omitempty"`
}

// zeroTraceID is the canonical 32-zero string the W3C trace-context
// invalid TraceID prints to. We use it for the X-Correlation-ID
// response header AND for the request-log field when no SpanContext
// is in scope, so the wire shape is byte-for-byte identical to what
// GoFr emitted before PR-7's internal optimization.
const zeroTraceID = "00000000000000000000000000000000"
const zeroSpanID = "0000000000000000"

func (rl *RequestLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m "+
		"%8d\u001B[38;5;8mµs\u001B[0m %s %s \n", rl.TraceID, colorForStatusCode(rl.Response), rl.Response, rl.ResponseTime, rl.Method, rl.URI)
}

func colorForStatusCode(status int) int {
	const (
		blue   = 34
		red    = 202
		yellow = 220
	)

	switch {
	case status >= 200 && status < 300:
		return blue
	case status >= 400 && status < 500:
		return yellow
	case status >= 500 && status < 600:
		return red
	}

	return 0
}

type logger interface {
	Log(...any)
	Error(...any)
}

// Logging is a middleware which logs response status and time in milliseconds along with other data.
//
// The StatusResponseWriter wrapper allocated per request is pooled in a
// closure-owned sync.Pool — the pool is constructed once per Logging()
// invocation (typically once per app) and tied to this middleware's
// lifetime, not the package, so we avoid a shared global. Reset() zeros
// the writer fields before Put so a stale ResponseWriter pointer can
// never leak across requests.
func Logging(probes LogProbes, logger logger) func(inner http.Handler) http.Handler {
	pool := sync.Pool{
		New: func() any { return &StatusResponseWriter{} },
	}

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			srw := pool.Get().(*StatusResponseWriter)
			srw.ResponseWriter = w
			srw.status = 0
			srw.wroteHeader = false

			defer func() {
				srw.ResponseWriter = nil
				srw.status = 0
				srw.wroteHeader = false
				pool.Put(srw)
			}()

			// Fetch SpanContext once and resolve trace/span IDs to strings only
			// when they are valid. Under a noop tracer (the default after PR-1
			// when no exporter is configured) the SpanContext is invalid and
			// the IDs are all-zeros — calling .String() on those is wasted
			// allocation. Substitute the precomputed zero-string constants so
			// the log line and the X-Correlation-ID response header carry
			// byte-identical values to the pre-PR-7 wire shape.
			sc := trace.SpanFromContext(r.Context()).SpanContext()

			var traceID, spanID string
			if sc.IsValid() {
				traceID = sc.TraceID().String()
				spanID = sc.SpanID().String()
			} else {
				traceID = zeroTraceID
				spanID = zeroSpanID
			}

			srw.Header().Set("X-Correlation-ID", traceID)

			defer func() { panicRecovery(recover(), srw, logger) }()

			// Skip logging for default probe paths if log probes are disabled.
			// time.Now() (vDSO call) is deferred past this so probe paths do
			// not pay for a timestamp that is then thrown away.
			if isLogProbeDisabled(probes, r.URL.Path) {
				inner.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			defer handleRequestLog(srw, r, start, traceID, spanID, logger)

			inner.ServeHTTP(srw, r)
		})
	}
}

func handleRequestLog(srw *StatusResponseWriter, r *http.Request, start time.Time, traceID, spanID string, logger logger) {
	l := &RequestLog{
		TraceID:      traceID,
		SpanID:       spanID,
		StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
		ResponseTime: time.Since(start).Nanoseconds() / 1000,
		Method:       r.Method,
		UserAgent:    r.UserAgent(),
		IP:           getIPAddress(r),
		URI:          r.RequestURI,
		Response:     srw.status,
	}

	if logger != nil {
		if srw.status >= http.StatusInternalServerError {
			logger.Error(l)
		} else {
			logger.Log(l)
		}
	}
}

// isLogProbeDisabled checks if probes are disabled to skip logging for default probe paths
// and additional health check paths of services.
func isLogProbeDisabled(probes LogProbes, urlPath string) bool {
	// if probes is not disabled, dont need to check for default probe paths
	if !probes.Disabled {
		return false
	}

	// check if urlPath is in the list of default probe paths and matches any of the values in the map
	for _, path := range probes.Paths {
		if urlPath == path && probes.Disabled {
			return true
		}
	}

	return false
}

func getIPAddress(r *http.Request) string {
	ips := strings.Split(r.Header.Get("X-Forwarded-For"), ",")

	// According to GCLB Documentation (https://cloud.google.com/load-balancing/docs/https/), IPs are added in following sequence.
	// X-Forwarded-For: <unverified IP(s)>, <immediate client IP>, <global forwarding rule external IP>, <proxies running in GCP>
	ipAddress := ips[0]

	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	return strings.TrimSpace(ipAddress)
}

type panicLog struct {
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

func panicRecovery(re any, w http.ResponseWriter, logger logger) {
	if re == nil {
		return
	}

	var e string

	switch t := re.(type) {
	case string:
		e = t
	case error:
		e = t.Error()
	default:
		e = "Unknown panic type"
	}

	logger.Error(panicLog{
		Error:      e,
		StackTrace: string(debug.Stack()),
	})

	w.WriteHeader(http.StatusInternalServerError)

	//nolint:goconst // JSON envelope keys (status/message), not shared constants
	res := map[string]any{
		"code":    http.StatusInternalServerError,
		"status":  "ERROR",
		"message": "Some unexpected error has occurred",
	}
	_ = json.NewEncoder(w).Encode(res)
}
