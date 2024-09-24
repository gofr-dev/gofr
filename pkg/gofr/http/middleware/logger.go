package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel/trace"
)

// StatusResponseWriter Defines own Response Writer to be used for logging of status - as http.ResponseWriter does not let us read status.
type StatusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *StatusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// RequestLog represents a log entry for HTTP requests.
type RequestLog struct {
	CorrelationID string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
	StartTime     string `json:"start_time,omitempty"`
	ResponseTime  int64  `json:"response_time,omitempty"`
	Method        string `json:"method,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	IP            string `json:"ip,omitempty"`
	URI           string `json:"uri,omitempty"`
	Response      int    `json:"response,omitempty"`
}

func (rl *RequestLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m "+
		"%8d\u001B[38;5;8mµs\u001B[0m %s %s \n", rl.CorrelationID,
		colorForStatusCode(rl.Response), rl.Response, rl.ResponseTime, rl.Method, rl.URI)
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
	Log(...interface{})
	Error(...interface{})
}

func getIDs(requestCtx context.Context) (hasTraceID bool, ctx context.Context, correlationID, spanID string) {
	requestSpan := trace.SpanFromContext(requestCtx).SpanContext()
	hasTraceID = requestSpan.HasTraceID()

	if hasTraceID {
		correlationID = requestSpan.TraceID().String()
		spanID = trace.SpanFromContext(requestCtx).SpanContext().SpanID().String()
	} else {
		correlationID = uuid.New().String()
	}

	// revive:disable
	//nolint:staticcheck // Using built in type as retrieving value from context in other package.
	requestCtx = context.WithValue(requestCtx, "X-Correlation-ID", correlationID)
	// revive:enable

	return hasTraceID, requestCtx, correlationID, spanID
}

// Logging is a middleware which logs response status and time in milliseconds along with other data.
func Logging(logger logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &StatusResponseWriter{ResponseWriter: w}

			var spanID string

			var hasTraceID bool

			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				hasTrace, ctx, corrID, id := getIDs(r.Context())
				r = r.Clone(ctx)
				correlationID = corrID
				hasTraceID = hasTrace
				spanID = id
			}

			srw.Header().Set("X-Correlation-ID", correlationID)

			defer func(res *StatusResponseWriter, req *http.Request) {
				l := &RequestLog{
					CorrelationID: correlationID,
					StartTime:     start.Format("2006-01-02T15:04:05.999999999-07:00"),
					ResponseTime:  time.Since(start).Nanoseconds() / 1000,
					Method:        req.Method,
					UserAgent:     req.UserAgent(),
					IP:            getIPAddress(req),
					URI:           req.RequestURI,
					Response:      res.status,
				}

				if hasTraceID {
					l.SpanID = spanID
				}

				if logger != nil {
					if res.status >= http.StatusInternalServerError {
						logger.Error(l)
					} else {
						logger.Log(l)
					}
				}
			}(srw, r)

			defer func() {
				panicRecovery(recover(), srw, logger)
			}()

			inner.ServeHTTP(srw, r)
		})
	}
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

	res := map[string]interface{}{"code": http.StatusInternalServerError, "status": "ERROR", "message": "Some unexpected error has occurred"}
	_ = json.NewEncoder(w).Encode(res)
}
