package remotelogger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/logging"
)

// Mock Logger (implements logging.Logger)
type mockLogger struct {
	mu        sync.Mutex
	messages  []string
	level     logging.Level
	changeCnt int32
}

func (m *mockLogger) record(prefix string, _ ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, prefix)
}

func (m *mockLogger) Debug(args ...any)                  { m.record("Debug", args...) }
func (m *mockLogger) Debugf(format string, args ...any)  { m.record("Debugf", args...) }
func (m *mockLogger) Log(args ...any)                    { m.record("Log", args...) }
func (m *mockLogger) Logf(format string, args ...any)    { m.record("Logf", args...) }
func (m *mockLogger) Info(args ...any)                   { m.record("Info", args...) }
func (m *mockLogger) Infof(format string, args ...any)   { m.record("Infof", args...) }
func (m *mockLogger) Notice(args ...any)                 { m.record("Notice", args...) }
func (m *mockLogger) Noticef(format string, args ...any) { m.record("Noticef", args...) }
func (m *mockLogger) Warn(args ...any)                   { m.record("Warn", args...) }
func (m *mockLogger) Warnf(format string, args ...any)   { m.record("Warnf", args...) }
func (m *mockLogger) Error(args ...any)                  { m.record("Error", args...) }
func (m *mockLogger) Errorf(format string, args ...any)  { m.record("Errorf", args...) }
func (m *mockLogger) Fatal(args ...any)                  { m.record("Fatal", args...) }
func (m *mockLogger) Fatalf(format string, args ...any)  { m.record("Fatalf", args...) }

func (m *mockLogger) ChangeLevel(level logging.Level) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.level = level
	atomic.AddInt32(&m.changeCnt, 1)
}

// Mock RemoteConfigurable Client
type mockClient struct {
	updateCalled int32
	lastConfig   map[string]any
}

func (m *mockClient) UpdateConfig(cfg map[string]any) {
	atomic.AddInt32(&m.updateCalled, 1)
	m.lastConfig = cfg
}

// Tests for httpRemoteConfig
func TestHttpRemoteConfig_RegisterAndStart(t *testing.T) {
	// mock HTTP server returning valid JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	time.Sleep(120 * time.Millisecond) // wait for one tick

	if atomic.LoadInt32(&client.updateCalled) == 0 {
		t.Fatalf("expected UpdateConfig to be called at least once")
	}

	if client.lastConfig["logLevel"] != "DEBUG" {
		t.Errorf("expected logLevel=DEBUG, got %v", client.lastConfig["logLevel"])
	}
}

func TestHttpRemoteConfig_InvalidJSON(t *testing.T) {
	// mock server returning invalid JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid-json`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	logger := &mockLogger{}
	cfg := NewHTTPRemoteConfig(server.URL, 50*time.Millisecond, logger).(*httpRemoteConfig)
	client := &mockClient{}
	cfg.Register(client)

	cfg.Start()
	time.Sleep(120 * time.Millisecond)

	if atomic.LoadInt32(&client.updateCalled) == 0 {
		t.Errorf("expected UpdateConfig to be called at least once even if response invalid")
	}
}

// Tests for remoteLogger (RemoteConfigurable)
func TestRemoteLogger_UpdateConfig_ChangesLevel(t *testing.T) {
	logger := &mockLogger{}
	r := &remoteLogger{
		Logger:       logger,
		currentLevel: logging.INFO,
	}

	cfg := map[string]any{"logLevel": "DEBUG"}
	r.UpdateConfig(cfg)

	if r.currentLevel != logging.DEBUG {
		t.Errorf("expected log level to change to DEBUG, got %v", r.currentLevel)
	}

	if atomic.LoadInt32(&logger.changeCnt) == 0 {
		t.Errorf("expected ChangeLevel to be called")
	}
}

func TestRemoteLogger_UpdateConfig_NoChange(t *testing.T) {
	logger := &mockLogger{}
	r := &remoteLogger{
		Logger:       logger,
		currentLevel: logging.INFO,
	}

	cfg := map[string]any{"logLevel": "INFO"}
	r.UpdateConfig(cfg)

	if atomic.LoadInt32(&logger.changeCnt) != 0 {
		t.Errorf("expected ChangeLevel not to be called when same level")
	}
}

func TestRemoteLogger_UpdateConfig_InvalidKey(t *testing.T) {
	logger := &mockLogger{}
	r := &remoteLogger{
		Logger:       logger,
		currentLevel: logging.INFO,
	}

	cfg := map[string]any{"invalidKey": "DEBUG"}
	r.UpdateConfig(cfg)

	if atomic.LoadInt32(&logger.changeCnt) != 0 {
		t.Errorf("expected no ChangeLevel when invalid key present")
	}
}
