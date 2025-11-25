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
	ResponseTimeHuman string `json:"response_time_human,omitempty"`
	Method       string `json:"method,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	IP           string `json:"ip,omitempty"`
	URI          string `json:"uri,omitempty"`
	Response     int    `json:"response,omitempty"`
}

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
func Logging(probes LogProbes, logger logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &StatusResponseWriter{ResponseWriter: w}
			traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
			spanID := trace.SpanFromContext(r.Context()).SpanContext().SpanID().String()

			srw.Header().Set("X-Correlation-ID", traceID)

			defer func() { panicRecovery(recover(), srw, logger) }()

			// Skip logging for default probe paths if log probes are disabled
			if isLogProbeDisabled(probes, r.URL.Path) {
				inner.ServeHTTP(w, r)
				return
			}

			defer handleRequestLog(srw, r, start, traceID, spanID, logger)

			inner.ServeHTTP(srw, r)
		})
	}
}

func handleRequestLog(srw *StatusResponseWriter, r *http.Request, start time.Time, traceID, spanID string, logger logger) {
    duration := time.Since(start)

    l := &RequestLog{
        TraceID:           traceID,
        SpanID:            spanID,
        StartTime:         start.Format("2006-01-02T15:04:05.999999999-07:00"),
        ResponseTime:      duration.Nanoseconds() / 1000,
        ResponseTimeHuman: duration.String(),
        Method:            r.Method,
        UserAgent:         r.UserAgent(),
        IP:                getIPAddress(r),
        URI:               r.RequestURI,
        Response:          srw.status,
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

	res := map[string]any{"code": http.StatusInternalServerError, "status": "ERROR", "message": "Some unexpected error has occurred"}
	_ = json.NewEncoder(w).Encode(res)
}
