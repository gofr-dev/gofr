package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/api/trace"
)

// Define own Response Writer to be used for logging of status - as http.ResponseWriter does not let us read status.
type StatusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *StatusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

type LogLine struct {
	ID           string
	StartTime    string
	ResponseTime int64
	Method       string
	UserAgent    string
	IP           string
	URI          string
	Response     int
}

func (l *LogLine) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

// Logging is a middleware which logs response status and time in microseconds along with other data.
func Logging(logger *log.Logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &StatusResponseWriter{ResponseWriter: w}

			defer func(res *StatusResponseWriter, req *http.Request) {
				l := LogLine{
					ID:           trace.SpanFromContext(r.Context()).SpanContext().TraceID.String(),
					StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
					ResponseTime: time.Since(start).Nanoseconds() / 1000,
					Method:       req.Method,
					UserAgent:    req.UserAgent(),
					IP:           getIPAddress(req),
					URI:          req.RequestURI,
					Response:     res.status,
				}
				if logger != nil {
					logger.Print(l)
				}
			}(srw, r)

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
