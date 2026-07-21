package gofr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
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
	}

	for _, tt := range tests {
		key, value := attributeToStringPair(tt.keyValue)
		assert.Equal(t, tt.expectedKey, key, "Key mismatch")
		assert.Equal(t, tt.expectedValue, value, "Value mismatch")
	}
}
