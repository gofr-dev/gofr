package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockHandlerForCORS struct {
	statusCode int
	response   string
}

// ServeHTTP is used for testing different panic recovery cases.
func (r *MockHandlerForCORS) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(r.statusCode)
	_, _ = w.Write([]byte(r.response))
}

func Test_CORS(t *testing.T) {
	handler := CORS()(&MockHandlerForCORS{statusCode: http.StatusFound, response: "Sample Response"})

	tests := []struct {
		method   string
		respBody string
		respCode int
	}{
		{http.MethodGet, "Sample Response", http.StatusFound},
		{http.MethodOptions, "", http.StatusOK},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(tc.method, "/hello", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "TEST[%d], Failed.\n", i)
		assert.Equal(t, "POST, GET, OPTIONS, PUT, DELETE", w.Header().Get("Access-Control-Allow-Methods"), "TEST[%d], Failed.\n", i)

		// Check if no other headers apart from the allowed headers are being set
		assert.Equal(t, 2, len(w.Header()), "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.respCode, w.Code, "TEST[%d], Failed.\n", i)
		assert.Equal(t, tc.respBody, w.Body.String(), "TEST[%d], Failed.\n", i)
	}
}
