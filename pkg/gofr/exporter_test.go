package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_ExportSpans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := logging.NewLogger(logging.INFO)
	exporter := NewExporter(server.URL, logger)

	tests := []struct {
		desc  string
		spans []sdktrace.ReadOnlySpan
	}{
		{"Empty Spans Slice", []sdktrace.ReadOnlySpan{}},
		{"Success case", provideSampleSpan(t)},
	}

	for i, tc := range tests {
		err := exporter.ExportSpans(t.Context(), tc.spans)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExportSpansError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	server.Close()

	exporter := NewExporter(server.URL, logging.NewLogger(logging.INFO))

	err := exporter.ExportSpans(t.Context(), provideSampleSpan(t))
	require.Error(t, err, "Expected error for failed request")
}

// TRACER_URL can carry credentials in its userinfo or query; a failed export must not write them to the log
// or put them in the returned error.
func Test_ExportSpans_RedactsEndpointCredentials(t *testing.T) {
	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer rejecting.Close()

	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed.Close()

	closedHost := strings.TrimPrefix(closed.URL, "http://")
	withCredentials := func(serverURL string) string {
		return strings.Replace(serverURL, "http://", "http://bob:S3CRETPASS@", 1) + "/api/spans?api_key=TOPSECRETKEY"
	}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		desc      string
		endpoint  string
		ctx       context.Context
		expErrs   []error
		expURLErr bool   // a failed request keeps returning a *url.Error
		expErrMsg string // a part of the returned error's text
		expLog    string // a part of the log line; empty when nothing is logged
	}{
		{desc: "collector rejects the export", endpoint: withCredentials(rejecting.URL), ctx: t.Context(),
			expErrs:   []error{errUnexpectedStatusCode},
			expErrMsg: "failed to post spans on 'http://REDACTED@",
			expLog:    "failed to post spans on 'http://REDACTED@"},
		{desc: "collector unreachable", endpoint: withCredentials(closed.URL), ctx: t.Context(), expURLErr: true,
			expErrMsg: `Post "http://REDACTED@` + closedHost + `/api/spans?REDACTED": dial tcp`,
			expLog:    "failed to create spans, error: Post"},
		{desc: "context canceled", endpoint: withCredentials(rejecting.URL), ctx: canceled, expURLErr: true,
			expErrs:   []error{context.Canceled},
			expErrMsg: `/api/spans?REDACTED": context canceled`,
			expLog:    "failed to create spans, error: Post"},
		{desc: "unparsable port", endpoint: "http://bob:S3CRETPASS@" + closedHost + "abc/api/spans?api_key=TOPSECRETKEY",
			ctx: t.Context(), expURLErr: true,
			expErrMsg: `failed to create HTTP request, error: parse "REDACTED@` + closedHost + `abc/api/spans?REDACTED"`},
		{desc: "control character", endpoint: withCredentials(closed.URL) + "\n", ctx: t.Context(), expURLErr: true,
			expErrMsg: `failed to create HTTP request, error: parse "REDACTED@`},
		{desc: "schemeless endpoint", endpoint: "bob:S3CRETPASS@" + closedHost + "/api/spans?api_key=TOPSECRETKEY",
			ctx: t.Context(), expURLErr: true,
			expErrMsg: `unsupported protocol scheme "REDACTED"`,
			expLog:    "unsupported protocol scheme"},
		{desc: "username-only token", endpoint: "http://bobS3CRETPASS@" + closedHost + "/api/spans", ctx: t.Context(),
			expURLErr: true,
			expErrMsg: `Post "http://REDACTED@` + closedHost + `/api/spans": dial tcp`,
			expLog:    "REDACTED@"},
		{desc: "query-only key", endpoint: closed.URL + "/api/spans?api_key=TOPSECRETKEY", ctx: t.Context(),
			expURLErr: true,
			expErrMsg: `Post "` + closed.URL + `/api/spans?REDACTED": dial tcp`,
			expLog:    "/api/spans?REDACTED"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var err error

			logs := testutil.StderrOutputForFunc(func() {
				exporter := NewExporter(tc.endpoint, logging.NewLogger(logging.INFO))
				err = exporter.ExportSpans(tc.ctx, provideSampleSpan(t))
			})

			var urlErr *url.Error

			require.Error(t, err)
			assert.Equal(t, tc.expURLErr, errors.As(err, &urlErr))

			for _, expErr := range tc.expErrs {
				require.ErrorIs(t, err, expErr)
			}

			for _, secret := range []string{"S3CRETPASS", "TOPSECRETKEY", "bob"} {
				assert.NotContains(t, logs, secret)
				assert.NotContains(t, err.Error(), secret)
			}

			assert.Contains(t, err.Error(), tc.expErrMsg)
			assert.Contains(t, logs, tc.expLog)
		})
	}
}

// A cause that repeats any part of the endpoint's userinfo or query is rewritten, and still matches its sentinel.
func Test_redactCause(t *testing.T) {
	const endpoint = "http://bob:S3CRETPASS@collector:4318/api/spans?api_key=TOP%2BSECRET&region=eu"

	tests := []struct {
		desc   string
		cause  error
		expMsg string
	}{
		{desc: "no credential in the cause", cause: context.Canceled, expMsg: "context canceled"},
		{desc: "username", cause: fmt.Errorf("user bob: %w", context.Canceled), expMsg: "user REDACTED: context canceled"},
		{desc: "password", cause: fmt.Errorf("auth S3CRETPASS: %w", context.Canceled),
			expMsg: "auth REDACTED: context canceled"},
		{desc: "escaped query value", cause: fmt.Errorf("key TOP%%2BSECRET: %w", context.Canceled),
			expMsg: "key REDACTED: context canceled"},
		{desc: "unescaped query value", cause: fmt.Errorf("key TOP+SECRET: %w", context.Canceled),
			expMsg: "key REDACTED: context canceled"},
		{desc: "whole query", cause: fmt.Errorf("query api_key=TOP%%2BSECRET&region=eu: %w", context.Canceled),
			expMsg: "query REDACTED: context canceled"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := redactCause(tc.cause, endpoint)

			assert.Equal(t, tc.expMsg, err.Error())
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// Any 2xx is a successful export (Zipkin-compatible collectors return 202); non-2xx is an error.
func Test_ExportSpans_Accepts2xx(t *testing.T) {
	tests := []struct {
		status  int
		wantErr bool
	}{
		{http.StatusOK, false},
		{http.StatusCreated, false},
		{http.StatusAccepted, false},
		{http.StatusNoContent, false},
		{http.StatusBadRequest, true},
		{http.StatusInternalServerError, true},
	}

	for _, tc := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))

		exporter := NewExporter(server.URL, logging.NewLogger(logging.INFO))
		err := exporter.ExportSpans(t.Context(), provideSampleSpan(t))

		if tc.wantErr {
			require.Error(t, err, "status %d should be an error", tc.status)
		} else {
			require.NoError(t, err, "status %d should succeed", tc.status)
		}

		server.Close()
	}
}

// A root span must not carry a parentId; a child span must reference its parent.
func Test_convertSpans_RootParentIDOmitted(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(t.Context()) }()

	tracer := tp.Tracer("test")

	ctx, root := tracer.Start(t.Context(), "root")
	_, child := tracer.Start(ctx, "child")
	child.End()
	root.End()

	got := convertSpans([]sdktrace.ReadOnlySpan{
		root.(sdktrace.ReadOnlySpan),
		child.(sdktrace.ReadOnlySpan),
	})

	require.Len(t, got, 2)
	assert.Empty(t, got[0].ParentID, "root span must have no parentId")
	assert.Equal(t, got[0].ID, got[1].ParentID, "child span must reference the root")
}

// Each span's localEndpoint carries its own service name, not the first span's — matters when one
// exporter batches spans from more than one service.
func Test_convertSpans_PerSpanServiceName(t *testing.T) {
	spanFor := func(svc string) sdktrace.ReadOnlySpan {
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", svc))),
		)
		defer func() { _ = tp.Shutdown(t.Context()) }()

		_, span := tp.Tracer("test").Start(t.Context(), "s")
		span.End()

		return span.(sdktrace.ReadOnlySpan)
	}

	got := convertSpans([]sdktrace.ReadOnlySpan{spanFor("svc-a"), spanFor("svc-b")})

	require.Len(t, got, 2)
	assert.Equal(t, "svc-a", got[0].LocalEndpoint["serviceName"])
	assert.Equal(t, "svc-b", got[1].LocalEndpoint["serviceName"], "second span must keep its own service name")
}

func provideSampleSpan(t *testing.T) []sdktrace.ReadOnlySpan {
	t.Helper()

	tp := sdktrace.NewTracerProvider()

	defer func(tp *sdktrace.TracerProvider, ctx context.Context) {
		err := tp.Shutdown(ctx)
		if err != nil {
			t.Error(err)
		}
	}(tp, t.Context())

	otel.SetTracerProvider(tp)

	tracer := otel.Tracer("test-tracer")

	_, span := tracer.Start(t.Context(), "test-span")
	span.End()

	ro := span.(sdktrace.ReadOnlySpan)

	return []sdktrace.ReadOnlySpan{ro}
}

func Test_attributeToStringPair(t *testing.T) {
	tests := []struct {
		name           string
		keyValue       attribute.KeyValue
		expectedKey    string
		expectedValue  string
		expectedErrMsg string
	}{
		{
			name:           "BoolSlice",
			keyValue:       attribute.BoolSlice("boolKey", []bool{true, false}),
			expectedKey:    "boolKey",
			expectedValue:  `[true,false]`,
			expectedErrMsg: "",
		},
		{
			name:           "Int64Slice",
			keyValue:       attribute.Int64Slice("int64Key", []int64{1, 2, 3}),
			expectedKey:    "int64Key",
			expectedValue:  `[1,2,3]`,
			expectedErrMsg: "",
		},
		{
			name:           "Float64Slice",
			keyValue:       attribute.Float64Slice("float64Key", []float64{1.1, 2.2, 3.3}),
			expectedKey:    "float64Key",
			expectedValue:  `[1.1,2.2,3.3]`,
			expectedErrMsg: "",
		},
		{
			name:           "StringSlice",
			keyValue:       attribute.StringSlice("stringKey", []string{"a", "b", "c"}),
			expectedKey:    "stringKey",
			expectedValue:  `["a","b","c"]`,
			expectedErrMsg: "",
		},
		{
			name:           "Bool",
			keyValue:       attribute.Bool("boolKey", true),
			expectedKey:    "boolKey",
			expectedValue:  "true",
			expectedErrMsg: "",
		},
		{
			name:           "Int64",
			keyValue:       attribute.Int64("int64Key", 123),
			expectedKey:    "int64Key",
			expectedValue:  "123",
			expectedErrMsg: "",
		},
		{
			name:           "Float64",
			keyValue:       attribute.Float64("float64Key", 1.23),
			expectedKey:    "float64Key",
			expectedValue:  "1.23",
			expectedErrMsg: "",
		},
		{
			name:           "String",
			keyValue:       attribute.String("stringKey", "stringValue"),
			expectedKey:    "stringKey",
			expectedValue:  "stringValue",
			expectedErrMsg: "",
		},
		{
			name:           "Empty",
			keyValue:       attribute.KeyValue{Key: "emptyKey"},
			expectedKey:    "emptyKey",
			expectedValue:  "invalid",
			expectedErrMsg: "",
		},
		{
			name:           "ByteSlice falls back to the value's string form",
			keyValue:       attribute.ByteSlice("bytesKey", []byte("hi")),
			expectedKey:    "bytesKey",
			expectedValue:  "aGk=",
			expectedErrMsg: "",
		},
	}

	for _, tt := range tests {
		key, value := attributeToStringPair(tt.keyValue)
		assert.Equal(t, tt.expectedKey, key, "Key mismatch")
		assert.Equal(t, tt.expectedValue, value, "Value mismatch")
	}
}

// Test_attributeToStringPair_StringFallbackTypes pins the value exported for the attribute
// types that have no JSON encoding of their own and use the value's String form.
func Test_attributeToStringPair_StringFallbackTypes(t *testing.T) {
	tests := []struct {
		name          string
		keyValue      attribute.KeyValue
		expectedValue string
	}{
		{
			name:          "Slice",
			keyValue:      attribute.Slice("sliceKey", attribute.StringValue("a"), attribute.Int64Value(1)),
			expectedValue: `["a",1]`,
		},
		{
			name:          "Map",
			keyValue:      attribute.Map("mapKey", attribute.String("k", "v"), attribute.Bool("b", true)),
			expectedValue: `{"b":true,"k":"v"}`,
		},
		{
			name:          "EmptyStringSlice",
			keyValue:      attribute.StringSlice("stringKey", nil),
			expectedValue: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value := attributeToStringPair(tt.keyValue)
			assert.Equal(t, string(tt.keyValue.Key), key)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func Test_ExportSpans_InvalidEndpoint(t *testing.T) {
	exporter := NewExporter("http://[::1", logging.NewMockLogger(logging.FATAL))

	err := exporter.ExportSpans(t.Context(), provideSampleSpan(t))

	require.ErrorContains(t, err, "failed to create HTTP request")
}

func Test_convertSpans_SpanAttributesBecomeTags(t *testing.T) {
	tp := sdktrace.NewTracerProvider()

	defer func() { _ = tp.Shutdown(t.Context()) }()

	_, span := tp.Tracer("test").Start(t.Context(), "s")
	span.SetAttributes(attribute.Int("http.status", 200), attribute.String("route", "/users"))
	span.End()

	got := convertSpans([]sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})

	require.Len(t, got, 1)
	assert.Equal(t, "200", got[0].Tags["http.status"])
	assert.Equal(t, "/users", got[0].Tags["route"])
}
