package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockHandlerForHeaderPropagation struct{}

// ServeHTTP is used for testing if the request context has traceId
func (r *MockHandlerForHeaderPropagation) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	authorization := req.Context().Value(AuthorizationHeader).(string)
	b3TraceID := req.Context().Value(B3TraceIDKey).(string)
	authorizationHeader := req.Context().Value(AuthenticatedUserIDKey).(string)
	body := strings.Join([]string{b3TraceID, authorizationHeader, authorization}, ",")

	_, _ = w.Write([]byte(body))
}

func TestPropagateHeaders(t *testing.T) {
	handler := PropagateHeaders(&MockHandlerForHeaderPropagation{})
	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Set("X-B3-TraceID", "WEB")
	req.Header.Set("Authorization", "gofr")
	req.Header.Set("X-Authenticated-UserId", "gofr.dev")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Body.String() != "WEB,gofr.dev,gofr" {
		t.Errorf("propagation of headers through context failed. Got %v\tExpected %v", recorder.Body.String(), "WEB,gofr.dev")
	}
}
