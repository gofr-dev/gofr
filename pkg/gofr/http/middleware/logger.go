package middleware

import (
	"encoding/json"
	"fmt"
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

type RequestLog struct {
	ID           string `json:"id,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	ResponseTime int64  `json:"response_time,omitempty"`
	Method       string `json:"method,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	IP           string `json:"ip,omitempty"`
	URI          string `json:"uri,omitempty"`
	Response     int    `json:"response,omitempty"`
}

type logger interface {
	Log(...interface{})
	Error(...interface{})
}

// Logging is a middleware which logs response status and time in microseconds along with other data.
func Logging(logger logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &StatusResponseWriter{ResponseWriter: w}
			reqID := GetCorrelationID(r)
			srw.Header().Set("X-Correlation-Id", reqID)

			defer func(res *StatusResponseWriter, req *http.Request) {
				l := RequestLog{
					ID:           reqID,
					StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
					ResponseTime: time.Since(start).Nanoseconds() / 1000,
					Method:       req.Method,
					UserAgent:    req.UserAgent(),
					IP:           getIPAddress(req),
					URI:          req.RequestURI,
					Response:     res.status,
				}
				if logger != nil {
					logger.Log(l)
				}
			}(srw, r)

			defer panicRecovery(srw, logger)

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

func panicRecovery(w http.ResponseWriter, logger logger) {
	re := recover()

	if re != nil {
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
}

func GetCorrelationID(r *http.Request) string {
	correlationIDFromRequest, err := trace.TraceIDFromHex(r.Header.Get("X-Correlation-Id"))
	if err != nil {
		correlationIDFromSpan := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		// if tracing is not enabled, otel sets the trace-ID to "00000000000000000000000000000000" (nil type of [16]byte)

		const correlationIDLength = 32

		nullCorrelationID := fmt.Sprintf("%0*s", correlationIDLength, "")

		if correlationIDFromSpan == nullCorrelationID {
			id, _ := uuid.NewUUID()
			s := strings.Split(id.String(), "-")

			correlationIDFromSpan = strings.Join(s, "")
		}

		return correlationIDFromSpan
	}

	return correlationIDFromRequest.String()
}
