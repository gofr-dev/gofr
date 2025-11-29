package remotelogger

import (
	// Standard library.
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// Third-party.
	"github.com/stretchr/testify/assert"

	// Local module.
	"gofr.dev/pkg/gofr/logging"
)

// Mock Logger (implements logging.Logger).
type mockLogger struct {
	mu        sync.Mutex
	messages  []string
	level     logging.Level
	changeCnt int32
}

func (m *mockLogger) record(prefix string, parts ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := fmt.Sprint(append([]any{prefix, ":"}, parts...)...)
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Debug(args ...any) { m.record("Debug", args...) }
func (m *mockLogger) Debugf(format string, args ...any) {
	m.record("Debugf", fmt.Sprintf(format, args...))
}
func (m *mockLogger) Log(args ...any)                 { m.record("Log", args...) }
func (m *mockLogger) Logf(format string, args ...any) { m.record("Logf", fmt.Sprintf(format, args...)) }
func (m *mockLogger) Info(args ...any)                { m.record("Info", args...) }
func (m *mockLogger) Infof(format string, args ...any) {
	m.record("Infof", fmt.Sprintf(format, args...))
}
func (m *mockLogger) Notice(args ...any) { m.record("Notice", args...) }
func (m *mockLogger) Noticef(format string, args ...any) {
	m.record("Noticef", fmt.Sprintf(format, args...))
}
func (m *mockLogger) Warn(args ...any) { m.record("Warn", args...) }
func (m *mockLogger) Warnf(format string, args ...any) {
	m.record("Warnf", fmt.Sprintf(format, args...))
}
func (m *mockLogger) Error(args ...any) { m.record("Error", args...) }
func (m *mockLogger) Errorf(format string, args ...any) {
	m.record("Errorf", fmt.Sprintf(format, args...))
}
func (m *mockLogger) Fatal(args ...any) { m.record("Fatal", args...) }
func (m *mockLogger) Fatalf(format string, args ...any) {
	m.record("Fatalf", fmt.Sprintf(format, args...))
}

func (m *mockLogger) ChangeLevel(level logging.Level) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.level = level
	atomic.AddInt32(&m.changeCnt, 1)
}

// Mock RemoteConfigurable Client.
type mockClient struct {
	updateCalled int32
	lastConfig   map[string]any
}

func (m *mockClient) UpdateConfig(cfg map[string]any) {
	atomic.AddInt32(&m.updateCalled, 1)
	m.lastConfig = cfg
}

// Tests for httpRemoteConfig.
func TestHttpRemoteConfig_RegisterAndStart(t *testing.T) {
	// mock HTTP server returning valid JSON.
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": map[string]string{
				"serviceName": "test-service",
				"logLevel":    "DEBUG",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	logger := &mockLogger{}
	cfg := NewHTTPRemoteConfig(server.URL, 50*time.Millisecond, logger).(*httpRemoteConfig)

	client := &mockClient{}
	cfg.Register(client)

	cfg.Start()

	// wait for at least one polling tick.
	time.Sleep(120 * time.Millisecond)

	updateCount := atomic.LoadInt32(&client.updateCalled)
	assert.Positive(t, updateCount, "UpdateConfig called")

	if client.lastConfig == nil {
		t.Fatalf("expected lastConfig not nil")
	}

	assert.Equal(t, "DEBUG", client.lastConfig["logLevel"], "logLevel value")
}

func TestHttpRemoteConfig_InvalidJSON_LogsErrorAndDoesNotUpdateClients(t *testing.T) {
	t.Parallel()

	// mock server returning invalid JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`invalid-json`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	logger := &mockLogger{}
	cfg := NewHTTPRemoteConfig(server.URL, 50*time.Millisecond, logger).(*httpRemoteConfig)
	client := &mockClient{}

	cfg.Register(client)
	cfg.Start()

	// Wait for a couple of polling intervals
	time.Sleep(200 * time.Millisecond)

	// Client must not be updated on invalid JSON.
	updateCount := atomic.LoadInt32(&client.updateCalled)
	assert.Equal(t, int32(0), updateCount, "client update count on invalid JSON")

	// Logger must have recorded an error message (case-insensitive/substring match).
	logger.mu.Lock()
	messagesCopy := append([]string(nil), logger.messages...)
	logger.mu.Unlock()

	// Look for "invalid" or "error" substring in logs.
	found := false

	for _, msg := range messagesCopy {
		lmsg := strings.ToLower(msg)
		if strings.Contains(lmsg, "invalid") || strings.Contains(lmsg, "error") {
			found = true
			break
		}
	}

	assert.True(t, found,
		"expected error log message for invalid JSON response, got logs: %v",
		messagesCopy,
	)
}

func TestRemoteLogger_UpdateConfig_Behaviors(t *testing.T) {
	cases := []struct {
		name          string
		startLevel    logging.Level
		cfg           map[string]any
		wantLevel     logging.Level
		wantChangeCnt func(t *testing.T, cnt int32)
	}{
		{
			name:       "change-to-debug",
			startLevel: logging.INFO,
			cfg:        map[string]any{"logLevel": "DEBUG"},
			wantLevel:  logging.DEBUG,
			wantChangeCnt: func(t *testing.T, _ int32) {
				t.Helper() // REQUIRED by thelper
			},
		},
		{
			name:       "no-change-same-level",
			startLevel: logging.INFO,
			cfg:        map[string]any{"logLevel": "INFO"},
			wantLevel:  logging.INFO,
			wantChangeCnt: func(t *testing.T, cnt int32) {
				t.Helper() // REQUIRED by thelper
				assert.Equal(t, int32(0), cnt, "expected no change")
			},
		},
		{
			name:       "invalid-key",
			startLevel: logging.INFO,
			cfg:        map[string]any{"invalidKey": "DEBUG"},
			wantLevel:  logging.INFO,
			wantChangeCnt: func(t *testing.T, cnt int32) {
				t.Helper() // REQUIRED by thelper
				assert.Equal(t, int32(0), cnt, "expected no change for invalid key")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := &mockLogger{}
			r := &remoteLogger{
				Logger:       logger,
				currentLevel: tc.startLevel,
			}

			r.UpdateConfig(tc.cfg)

			assert.Equal(t, tc.wantLevel, r.currentLevel, "current level mismatch")
			tc.wantChangeCnt(t, atomic.LoadInt32(&logger.changeCnt))
		})
	}
}
