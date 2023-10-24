package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

func getAppData(ctx context.Context) map[string]interface{} {
	appData := make(map[string]interface{})

	if data, ok := ctx.Value(LogDataKey("appLogData")).(*sync.Map); ok {
		data.Range(func(key, value interface{}) bool {
			if k, ok := key.(string); ok {
				appData[k] = value
			}

			return true
		})
	}

	return appData
}

// Recover handles error by allowing the inner HTTP handler to recover from panics.
func Recover(logger logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer panicRecovery(logger, w, r)
			inner.ServeHTTP(w, r)
		})
	}
}

func panicRecovery(logger logger, w http.ResponseWriter, r *http.Request) {
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
		// fetch the appData from request context and generate a map of type map[string]interface{}, if appData is nil
		// then getAppData will return empty map
		data := getAppData(r.Context())
		// fetch the correlationID from request context and add it in data
		if correlationID := r.Context().Value(CorrelationIDKey); correlationID != nil {
			data[string(CorrelationIDKey)] = correlationID
		}

		logger.Errorf("Req: %s %s Panic:  %v", r.Method, r.RequestURI, e, data)

		// In case of errors, report to new relic
		newRelicError(r.Context(), errors.New(e))

		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		// remove the trailing slash
		path = strings.TrimSuffix(path, "/")
		// pushing error type to prometheus in case of panic
		ErrorTypesStats.With(prometheus.Labels{"type": "PANIC", "path": path, "method": r.Method}).Inc()

		err := FetchErrResponseWithCode(http.StatusInternalServerError, "some unexpected error has occurred", "PANIC")

		ErrorResponse(w, r, nil, *err)
	}
}
