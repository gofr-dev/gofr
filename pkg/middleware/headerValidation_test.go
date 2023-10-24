package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type validatorHandler struct{}

func (h validatorHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Response == nil {
		w.WriteHeader(http.StatusOK)

		return
	}

	w.WriteHeader(req.Response.StatusCode)
	resp, _ := io.ReadAll(req.Response.Body)

	_, _ = w.Write(resp)
}

type MultipleErrors struct {
	StatusCode int               `json:"-" xml:"-"`
	Errors     []errors.Response `json:"errors" xml:"errors"`
}

func Test_isValidZopsmartTenant(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"invalid X-Zopsmart-Tenant value", "test", false},
		{"empty X-Zopsmart-Tenant value", "", false},
		{"valid X-Zopsmart-Tenant value", "good4more", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidZopsmartTenant(tt.val); got != tt.want {
				t.Errorf("isValidZopsmartTenant() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isValidTrueClientIP(t *testing.T) {
	tests := []struct {
		ip      string
		isValid bool
	}{
		{"", false},
		{"2,3,4,5", false},
		{"1. .222222.1", false},
		{"1.1.1b2.1", false},
		{"8.8.8.8", true},
		{"-1.-2.-3.-4", false},
		{"2001:db8:85a3:8d3:1319:8a2e:370:7348", true},
		{"2001::85a3:8d3:1319:8a2e:370:7348", true},
		{"2001::85a3:8g3:1319:8a2e:370:7348", false},
		{"localhost", false},
		{"http://some-random-url", false},
	}

	for i, tt := range tests {
		isValid := isValidTrueClientIP(tt.ip)
		if tt.isValid != isValid {
			t.Errorf("Testcase[%v]: expected: %v\nGot: %v\n", i, tt.isValid, isValid)
		}
	}
}

//nolint:gocognit // breaking the test function will reduce readability.
func Test_validateMandatoryHeaders(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	tests := []struct {
		name       string
		headers    map[string]string
		envHeaders string
		errs       MultipleErrors
	}{
		{"X-Zopsmart-Tenant not passed", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"True-Client-Ip":         "127.0.0.1",
			"X-Correlation-ID":       "1s3d323adsd",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header X-Zopsmart-Tenant is missing"}}}},

		{"X-Zopsmart-Tenant invalid value passed", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"True-Client-Ip":         "127.0.0.1",
			"X-Correlation-ID":       "1s3d323adsd",
			"X-Zopsmart-Tenant":      "foods",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header X-Zopsmart-Tenant value is invalid"}}}},
		{"X-Correlation-ID not passed", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"True-Client-Ip":         "127.0.0.1",
			"X-Zopsmart-Tenant":      "good4more",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header X-Correlation-ID is missing"}}}},

		{"invalid True-Client-Ip", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"X-Correlation-ID":       "1s3d323adsd",
			"True-Client-Ip":         "127.0.0.1.1",
			"X-Zopsmart-Tenant":      "good4more",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header True-Client-Ip value is invalid"}}}},

		{"True-Client-Ip value NOT passed", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"X-Correlation-ID":       "1s3d323adsd",
			"X-Zopsmart-Tenant":      "good4more",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header True-Client-Ip is missing"}}}},
		{"invalid X-Zopsmart-Tenant", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"True-Client-Ip":         "127.0.0.1",
			"X-B3-TraceID":           "1s3d323adsd",
			"X-Zopsmart-Tenant":      "good14less",
		}, "", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header X-Zopsmart-Tenant value is invalid"}}}},
		{"env headers not present", map[string]string{
			"X-Authenticated-UserId": "gofr0000",
			"True-Client-Ip":         "127.0.0.1",
			"X-B3-TraceID":           "1s3d323adsd",
			"X-Zopsmart-Tenant":      "good4more",
		}, "Test-Header", MultipleErrors{StatusCode: http.StatusBadRequest, Errors: []errors.Response{
			{Code: "BAD_REQUEST", Reason: "Header Test-Header is missing"}}}},
	}

	for i, tt := range tests {
		tt := tt
		j := i

		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)

			for k, v := range tests[j].headers {
				req.Header.Set(k, v)
			}

			h := ValidateHeaders(tests[j].envHeaders, logger)(validatorHandler{})

			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			var err MultipleErrors
			_ = json.Unmarshal(w.Body.Bytes(), &err)

			if w.Code != tt.errs.StatusCode {
				t.Errorf("expected status code: %v\tgot %v\n", tt.errs.StatusCode, w.Code)
			}

			if err.Errors[0].Code != tt.errs.Errors[0].Code {
				t.Errorf("expected status code: %v\tgot %v\n", tt.errs.Errors[0].Code, err.Errors[0].Code)
			}

			if err.Errors[0].Reason != tt.errs.Errors[0].Reason {
				t.Errorf("expected reaseon: %v\tgot %v\n", tt.errs.Errors[0].Reason, err.Errors[0].Reason)
			}
			// Check if error is being logged
			if !strings.Contains(b.String(), tt.errs.Errors[0].Reason) {
				t.Errorf("Middleware Error is not logged")
			}
		})
	}
}

//nolint:gocognit // We are testing multiple headers here.
func Test_HeaderValidation_Success(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)

	headers := map[string]string{
		"X-Authenticated-UserId": "gofr0000",
		"True-Client-Ip":         "127.0.0.1",
		"X-B3-TraceID":           "1s3d323adsd",
		"Test-Header":            "test"}

	req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	xZopsmartTenantVals := []string{"good4more", "Zopsmart"} // for checking case insensitivity of X-Zopsmart-Tenant

	for _, v := range xZopsmartTenantVals {
		req.Header.Set("X-Zopsmart-Tenant", v)

		h := ValidateHeaders("", logger)(validatorHandler{})

		w := new(httptest.ResponseRecorder)

		h.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected %v\tgot %v", http.StatusOK, w.Code)
		}

		sts := w.Header().Get("Strict-Transport-Security")
		csp := w.Header().Get("Content-Security-Policy")
		xcto := w.Header().Get("X-Content-Type-Options")
		xssp := w.Header().Get("X-XSS-Protection")

		if sts != "max-age=86400; includeSubDomains" || csp != "default-src 'self'; script-src 'self'" || xcto != "nosniff" || xssp != "1" {
			t.Errorf("invalid set of response headers\n"+
				"Got: \n%v:%v, \n%v:%v, \n%v:%v, \n%v:%v", "Strict-Transport-Security", sts, "Content-Security-Policy", csp,
				"X-Content-Type-Options", xcto, "X-XSS-Protection", xssp)
		}
	}
}

func Test_ExemptPath(t *testing.T) {
	tests := []struct {
		r          *http.Request
		isExempted bool
	}{
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/v1/metrics"}}, true},
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/dummy"}}, false},
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/v1/.well-known/heartbeat"}}, true},
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/v3/.well-known/swagger"}}, true},
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/metrics"}}, true},
		{&http.Request{URL: &url.URL{Host: "http://localhost:8000", Path: "/v2/.well-known/openapi.json"}}, true},
	}
	for i, tt := range tests {
		tt := tt
		j := i
		t.Run(strconv.Itoa(j), func(t *testing.T) {
			isExempted := ExemptPath(tt.r)

			if isExempted != tt.isExempted {
				t.Errorf("Failed[%v]: expected %v \n Got %v ", j, tt.isExempted, isExempted)
			}
		})
	}
}
