package gofr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// TestQueryContentTypeGuard_LogsAtDeclaredLevel pins the operational contract
// the reviewer flagged: a client's bad Content-Type is a 4xx and MUST NOT show
// up as an ERROR log line. ErrorMissingParam and ErrorUnsupportedMediaType
// both declare LogLevel INFO; the guard must dispatch to log.Info, not
// log.Error, or every missing-header request pages the on-call.
//
// The MockLogger writes ERROR to stderr and everything else to stdout, so a
// leak into stderr is what a regression would look like here.
func TestQueryContentTypeGuard_LogsAtDeclaredLevel(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantLevel   string
		// ERROR (and above) is written to stderr; everything else to stdout.
		onStderr bool
	}{
		{"missing-CT-is-INFO", "", "INFO", false},
		{"unsupported-CT-is-INFO", "text/plain", "INFO", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serve := func() {
				inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				guard := queryContentTypeGuard(inner, logging.NewLogger(logging.DEBUG))

				req := httptest.NewRequestWithContext(t.Context(), MethodQuery, "/x", strings.NewReader(`{}`))
				if tc.contentType != "" {
					req.Header.Set("Content-Type", tc.contentType)
				}

				guard.ServeHTTP(httptest.NewRecorder(), req)
			}

			logs := testutil.StdoutOutputForFunc(serve)
			if tc.onStderr {
				logs = testutil.StderrOutputForFunc(serve)
			}

			assert.Contains(t, logs, tc.wantLevel,
				"guard must log a 4xx at the level the error declares, not ERROR — "+
					"a false ERROR pages the on-call on every bad Content-Type")

			// The opposite stream must be empty: a leak into ERROR is the
			// regression the reviewer's fix prevents.
			if tc.onStderr {
				assert.Empty(t, testutil.StdoutOutputForFunc(serve))
			} else {
				assert.Empty(t, testutil.StderrOutputForFunc(serve))
			}
		})
	}
}

// TestQueryContentTypeGuard_Accept_QueryOnUnsupportedOnly pins RFC 10008 §3.1:
// a 415 advertises the accepted media types via Accept-Query. A 400 (missing
// header) MUST NOT — nothing about the media type is wrong, so nothing about
// media types belongs in the response.
func TestQueryContentTypeGuard_Accept_QueryOnUnsupportedOnly(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	guard := queryContentTypeGuard(inner, logging.NewLogger(logging.DEBUG))

	tests := []struct {
		name            string
		contentType     string
		wantStatus      int
		wantAcceptQuery bool
	}{
		{"415-advertises-accepted-types", "text/plain", http.StatusUnsupportedMediaType, true},
		{"400-does-not-advertise", "", http.StatusBadRequest, false},
		{"200-does-not-advertise", "application/json", http.StatusOK, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), MethodQuery, "/x", strings.NewReader(`{}`))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)

			acceptQuery := rec.Header().Get("Accept-Query")
			if tc.wantAcceptQuery {
				assert.Equal(t, gofrHTTP.AcceptedQueryMediaTypes(), acceptQuery,
					"415 must advertise the accepted media types (RFC 10008 §3.1)")
			} else {
				assert.Empty(t, acceptQuery,
					"Accept-Query belongs only on 415; %d is not a media-type refusal", tc.wantStatus)
			}
		})
	}
}
