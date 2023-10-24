package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

//nolint:gocognit // all the conditions are required for tests
func TestErrorResponse(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	req := httptest.NewRequest("GET", "http://dummy", nil)
	req.Header.Set("X-Authenticated-UserId", "gofr0000")
	req.Header.Set("True-Client-Ip", "localhost")
	req.Header.Set("X-Correlation-ID", "1s3d323adsd")
	req = req.Clone(context.WithValue(req.Context(), CorrelationIDKey, "gofrTest"))

	contentTypes := []string{"application/json", "application/xml", "text/xml", "text/plain"}
	sampleError := errors.MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []error{&errors.Response{
		Code: "BAD_REQUEST", Reason: "Header X-Zopsmart-Tenant is missing"}}}

	for i := range contentTypes {
		req.Header.Set("Content-Type", contentTypes[i])

		w := new(httptest.ResponseRecorder)

		ErrorResponse(w, req, logger, sampleError)

		if w.Code != http.StatusBadRequest && w.Header().Get("Content-Type") != contentTypes[i] {
			t.Errorf("expected %v\tgot %v", http.StatusBadRequest, w.Code)
		}
	}

	// case of some other Content-Type
	req.Header.Set("Content-Type", "application/postscript")

	w := new(httptest.ResponseRecorder)

	ErrorResponse(w, req, logger, sampleError)

	if w.Code != http.StatusBadRequest && w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected %v\tgot %v", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(b.String(), `"correlationId":"gofrTest"`) {
		t.Errorf("correlationID is not logged")
	}

	// Testing middleware log
	if !strings.Contains(b.String(), "Header X-Zopsmart-Tenant is missing") {
		t.Errorf("Middleware Error is not logged")
	}
}

//nolint:gocognit // all conditions are required for the test
func Test_fetchErrResponseWithCode(t *testing.T) {
	zone, _ := time.Now().Zone()

	tcs := []struct {
		statusCode int
		reason     string
		code       string
	}{
		{http.StatusUnauthorized, "UnAuthorised", "401"},
		{http.StatusInternalServerError, "Internal Server Error", "PANIC"},
	}

	for _, tc := range tcs {
		err := FetchErrResponseWithCode(tc.statusCode, tc.reason, tc.code)
		if err == nil {
			t.Errorf("Expected not nil, got nil")
			continue
		}

		if err.StatusCode != tc.statusCode {
			t.Errorf("Expected status code: %v, got: %v", tc.statusCode, err.StatusCode)
		}

		if len(err.Errors) != 1 {
			t.Errorf("Expected Errors size 1, got %d", len(err.Errors))
		} else if errorResponse, ok := err.Errors[0].(*errors.Response); ok {
			if errorResponse.Code != tc.code {
				t.Errorf("Expected Code %v, got %v", tc.code, errorResponse.Code)
			}
			if errorResponse.Reason != tc.reason {
				t.Errorf("Expected Reason %v, got %v", tc.reason, errorResponse.Reason)
			}
			if errorResponse.DateTime.Value != time.Now().Format(time.RFC3339) {
				t.Errorf("Expected TimeValue %v, got %v", time.Now().Format(time.RFC3339), errorResponse.DateTime.Value)
			}
			if errorResponse.DateTime.TimeZone != zone {
				t.Errorf("Expected TimeZone %v, got %v", zone, errorResponse.DateTime.TimeZone)
			}
		} else {
			t.Errorf("Expected error.Errors[0] to be of type *errors.Response, got: %T", err.Errors)
		}
	}
}
