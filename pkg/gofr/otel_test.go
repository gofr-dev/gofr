package gofr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single header",
			input: "Key=Value",
			expected: map[string]string{
				"Key": "Value",
			},
		},
		{
			name:  "multiple headers",
			input: "K1=V1,K2=V2",
			expected: map[string]string{
				"K1": "V1",
				"K2": "V2",
			},
		},
		{
			name:  "value with equals sign",
			input: "Hash=sha256=abc123,Key=value",
			expected: map[string]string{
				"Hash": "sha256=abc123",
				"Key":  "value",
			},
		},
		{
			name:  "skip invalid entries",
			input: "NoEquals,Valid=value,=EmptyKey",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "trim whitespace",
			input: " Key1 = Value1 , Key2 = Value2 ",
			expected: map[string]string{
				"Key1": "Value1",
				"Key2": "Value2",
			},
		},
		{
			name:  "empty key",
			input: "=Value,Valid=value",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "empty value",
			input: "Key=,Valid=value",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "base64 authorization header",
			input: "Authorization=Basic dXNlcjpwYXNz",
			expected: map[string]string{
				"Authorization": "Basic dXNlcjpwYXNz",
			},
		},
		{
			name:  "multiple headers with special characters",
			input: "X-Api-Key=abc123xyz,Authorization=Bearer token123,X-Scope-OrgID=tenant-1",
			expected: map[string]string{
				"X-Api-Key":     "abc123xyz",
				"Authorization": "Bearer token123",
				"X-Scope-OrgID": "tenant-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHeaders(tt.input)

			require.Equal(t, tt.expected, result)
		})
	}
}

func TestApp_getTracerHeaders_WithTracerHeaders(t *testing.T) {
	tests := []struct {
		name              string
		tracerHeaders     string
		expectedHeaders   map[string]string
		expectedHeaderLen int
	}{
		{
			name:              "multiple headers",
			tracerHeaders:     "X-Api-Key=secret123,Authorization=Bearer token",
			expectedHeaderLen: 2,
			expectedHeaders: map[string]string{
				"X-Api-Key":     "secret123",
				"Authorization": "Bearer token",
			},
		},
		{
			name:              "single header",
			tracerHeaders:     "X-Honeycomb-Team=abc123",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"X-Honeycomb-Team": "abc123",
			},
		},
		{
			name:              "priority over TRACER_AUTH_KEY",
			tracerHeaders:     "X-Custom-Header=value",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"X-Custom-Header": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configData := map[string]string{
				"TRACER_HEADERS": tt.tracerHeaders,
			}

			app := &App{
				Config: config.NewMockConfig(configData),
			}

			headers := app.getTracerHeaders()

			require.Equal(t, tt.expectedHeaders, headers)
			require.Len(t, headers, tt.expectedHeaderLen)
		})
	}
}

func TestApp_getTracerHeaders_WithAuthKey(t *testing.T) {
	tests := []struct {
		name              string
		tracerAuthKey     string
		expectedHeaders   map[string]string
		expectedHeaderLen int
	}{
		{
			name:              "backward compatibility",
			tracerAuthKey:     "Bearer legacy-token",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"Authorization": "Bearer legacy-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configData := map[string]string{
				"TRACER_AUTH_KEY": tt.tracerAuthKey,
			}

			app := &App{
				Config: config.NewMockConfig(configData),
			}

			headers := app.getTracerHeaders()

			require.Equal(t, tt.expectedHeaders, headers)
			require.Len(t, headers, tt.expectedHeaderLen)
		})
	}
}

func TestApp_getTracerHeaders_NoConfig(t *testing.T) {
	app := &App{
		Config: config.NewMockConfig(map[string]string{}),
	}

	headers := app.getTracerHeaders()

	require.Empty(t, headers)
}

var (
	errOtelStatus200 = errors.New("rpc error: code = Unknown desc = status 200")
	errOtelStatus204 = errors.New("rpc error: status 204")
	errOtelStatus201 = errors.New("status 201: ok")
	errOtelStatus500 = errors.New("rpc error: status 500")
)

type captureLogger struct {
	loggedErrors []string
}

func (*captureLogger) Debug(_ ...any)             {}
func (*captureLogger) Debugf(_ string, _ ...any)  {}
func (*captureLogger) Log(_ ...any)               {}
func (*captureLogger) Logf(_ string, _ ...any)    {}
func (*captureLogger) Info(_ ...any)              {}
func (*captureLogger) Infof(_ string, _ ...any)   {}
func (*captureLogger) Notice(_ ...any)            {}
func (*captureLogger) Noticef(_ string, _ ...any) {}
func (*captureLogger) Warn(_ ...any)              {}
func (*captureLogger) Warnf(_ string, _ ...any)   {}
func (l *captureLogger) Error(args ...any) {
	// otelErrorHandler passes a single string arg
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			l.loggedErrors = append(l.loggedErrors, s)
			return
		}
	}

	l.loggedErrors = append(l.loggedErrors, "non-string error")
}
func (*captureLogger) Errorf(_ string, _ ...any)   {}
func (*captureLogger) Fatal(_ ...any)              {}
func (*captureLogger) Fatalf(_ string, _ ...any)   {}
func (*captureLogger) ChangeLevel(_ logging.Level) {}

func TestOtelErrorHandler_Ignores2xxStatusErrors(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(errOtelStatus200)
	h.Handle(errOtelStatus204)
	h.Handle(errOtelStatus201)

	require.Empty(t, cl.loggedErrors)
}

func TestOtelErrorHandler_LogsNon2xxErrors(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(errOtelStatus500)

	require.Len(t, cl.loggedErrors, 1)
	require.Equal(t, "rpc error: status 500", cl.loggedErrors[0])
}

func TestOtelErrorHandler_NilErrorNoop(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(nil)

	require.Empty(t, cl.loggedErrors)
}
