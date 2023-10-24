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
	channel := req.Context().Value(ZopsmartChannelKey).(string)
	tenant := req.Context().Value(ZopsmartTenantKey).(string)
	body := strings.Join([]string{b3TraceID, authorizationHeader, authorization, channel, tenant}, ",")

	_, _ = w.Write([]byte(body))
}

func TestPropagateHeaders(t *testing.T) {
	handler := PropagateHeaders(&MockHandlerForHeaderPropagation{})
	req := httptest.NewRequest("GET", "/dummy", http.NoBody)
	req.Header.Set("X-B3-TraceID", "WEB")
	req.Header.Set("Authorization", "zop")
	req.Header.Set("X-Authenticated-UserId", "zopsmart")
	req.Header.Set("X-Zopsmart-Channel", "channel")
	req.Header.Set("X-Zopsmart-Tenant", "Tenant")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Body.String() != "WEB,zopsmart,zop,channel,Tenant" {
		t.Errorf("propagation of headers through context failed. Got %v\tExpected %v", recorder.Body.String(), "WEB,zopsmart")
	}
}
