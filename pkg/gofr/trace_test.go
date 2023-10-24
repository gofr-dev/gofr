package gofr

import (
	"errors"
	"io"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/stretchr/testify/assert"

	gofrErr "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func TestTraceExporterSuccess(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")
	err := tracerProvider(cfg, logger)

	assert.NoError(t, err)
}

func TestTraceExporterFailure(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)

	testcases := []struct {
		desc        string
		exporter    string
		url         string
		appName     string
		expectedErr error
	}{
		{"when exporter is neither zipkin nor gcp", "not zipkin", "http://localhost/9411", "gofr", gofrErr.Error("invalid exporter")},
		{"when exporter is zipkin", "zipkin", "invalid url", "gofr", errors.New("invalid collector " +
			"URL \"invalid url/api/v2/spans\": no scheme or host")},
		{"when exporter is gcp", "gcp", "http://fakeProject/9411", "sample-api", errors.New("stackdriver: " +
			"google: error getting credentials using GOOGLE_APPLICATION_CREDENTIALS environment variable: open secretkey.json: " +
			"no such file or directory")},
	}

	for i, tc := range testcases {
		cfg := &config.MockConfig{Data: map[string]string{
			"TRACER_EXPORTER": tc.exporter,
			"TRACER_URL":      tc.url,
		}}

		err := tracerProvider(cfg, logger)

		assert.Equal(t, tc.expectedErr, err, "Test[%d],Failed[%v]", i, tc.desc)
	}
}

func TestGetZipkinExporter(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	testcases := []struct {
		desc         string
		sampler      trace.Sampler
		alwaysSample string
		expTrace     *trace.TracerProvider
	}{
		{"URL:valid,SAMPLING:AlwaysSample", trace.AlwaysSample(), "true", &trace.TracerProvider{}},
		{"URL:valid,SAMPLING:ParentBased", trace.ParentBased(trace.TraceIDRatioBased(0.1)), "false", &trace.TracerProvider{}},
	}

	for i, tc := range testcases {
		cfg := &config.MockConfig{Data: map[string]string{
			"TRACER_EXPORTER":      "zipkin",
			"TRACER_URL":           "http://localhost/9411",
			"TRACER_ALWAYS_SAMPLE": tc.alwaysSample,
		}}

		e := &exporter{url: "http://localhost/9411"}

		tracers, err := e.getZipkinExporter(cfg, logger)

		assert.IsTypef(t, tc.expTrace, tracers, "Test[%d],failed:%v", i, tc.desc)
		assert.Nil(t, err, "Test[%d],failed:%v", i, tc.desc)
	}
}

func TestGetZipkinExporter_Fail(t *testing.T) {
	cfg := &config.MockConfig{Data: map[string]string{
		"TRACER_EXPORTER": "zipkin",
		"TRACER_URL":      "invalid url",
	}}

	e := &exporter{url: "invalid url"}

	tracers, err := e.getZipkinExporter(cfg, log.NewMockLogger(io.Discard))

	assert.Nil(t, tracers, "invalid URL")
	assert.Error(t, errors.New(""), err, "invalid URL")
}

func TestGetGCPExporter_Fail(t *testing.T) {
	cfg := &config.MockConfig{Data: map[string]string{
		"TRACER_EXPORTER": "zipkin",
		"TRACER_URL":      "invalid url",
	}}

	tracers, err := getGCPExporter(cfg, log.NewMockLogger(io.Discard))

	assert.Nil(t, tracers, "invalid URL")
	assert.Error(t, err, "invalid URL")
}

func Test_getResource(t *testing.T) {
	mockconfig := &MockConfig{GetOrDefaultFunc: func(key, defaultValue string) string {
		if key == "APP_VERSION" {
			return "TestVersion"
		} else if key == "APP_NAME" {
			return "TestApp"
		}
		return defaultValue
	}, GetFunc: func(string2 string) string {
		return ""
	}}

	r, err := getResource(mockconfig)

	expectedAttributes := []attribute.KeyValue{
		attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
		attribute.String(string(semconv.TelemetrySDKVersionKey), "TestVersion"),
		attribute.String(string(semconv.ServiceNameKey), "TestApp"),
	}

	assert.NoError(t, err, "Tests failed")
	assert.ElementsMatch(t, expectedAttributes, r.Attributes())
}

type MockConfig struct {
	GetOrDefaultFunc func(key, defaultValue string) string
	GetFunc          func(string) string
}

func (m *MockConfig) GetOrDefault(key, defaultValue string) string {
	return m.GetOrDefaultFunc(key, defaultValue)
}

func (m *MockConfig) Get(string) string {
	return ""
}
