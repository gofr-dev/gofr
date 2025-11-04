package config

import (
	"sync"
	"testing"
	"time"
)

// mockRemoteConfigurable is a mock that records updates for verification
type mockRemoteConfigurable struct {
	mu           sync.Mutex
	updatedCount int
	lastConfig   map[string]any
}

func (m *mockRemoteConfigurable) UpdateConfig(cfg map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedCount++
	m.lastConfig = cfg
}

// mockRemoteConfiguration simulates a runtime configuration manager
type mockRemoteConfiguration struct {
	mu           sync.Mutex
	registered   []RemoteConfigurable
	started      bool
	startTrigger chan map[string]any
}

func newMockRemoteConfiguration() *mockRemoteConfiguration {
	return &mockRemoteConfiguration{
		startTrigger: make(chan map[string]any, 1),
	}
}

func (m *mockRemoteConfiguration) Register(c RemoteConfigurable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registered = append(m.registered, c)
}

func (m *mockRemoteConfiguration) Start() {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	go func() {
		for cfg := range m.startTrigger {
			m.mu.Lock()
			for _, r := range m.registered {
				r.UpdateConfig(cfg)
			}
			m.mu.Unlock()
		}
	}()
}

func (m *mockRemoteConfiguration) pushConfig(cfg map[string]any) {
	m.startTrigger <- cfg
}

func TestRemoteConfiguration_RegisterAndStart(t *testing.T) {
	rc := newMockRemoteConfiguration()
	c1 := &mockRemoteConfigurable{}
	c2 := &mockRemoteConfigurable{}

	rc.Register(c1)
	rc.Register(c2)

	rc.Start()

	// Simulate runtime config update
	cfg := map[string]any{"log_level": "DEBUG"}
	rc.pushConfig(cfg)

	// Wait briefly for goroutine to deliver update
	time.Sleep(50 * time.Millisecond)

	if c1.updatedCount != 1 || c2.updatedCount != 1 {
		t.Fatalf("expected both configurables to be updated once, got c1=%d, c2=%d", c1.updatedCount, c2.updatedCount)
	}

	if c1.lastConfig["log_level"] != "DEBUG" {
		t.Errorf("expected c1 to receive correct config value, got %+v", c1.lastConfig)
	}

	if !rc.started {
		t.Errorf("expected configuration Start() to set started=true")
	}
}

func TestRemoteConfiguration_NoRegisteredComponents(t *testing.T) {
	rc := newMockRemoteConfiguration()
	rc.Start()

	cfg := map[string]any{"feature": "enabled"}
	rc.pushConfig(cfg)

	// Should not panic even if no components are registered
	time.Sleep(20 * time.Millisecond)
}

func TestRemoteConfigurable_UpdateConfigCalledMultipleTimes(t *testing.T) {
	rc := newMockRemoteConfiguration()
	c := &mockRemoteConfigurable{}
	rc.Register(c)
	rc.Start()

	for i := 0; i < 3; i++ {
		rc.pushConfig(map[string]any{"version": i})
	}

	time.Sleep(100 * time.Millisecond)

	if c.updatedCount != 3 {
		t.Fatalf("expected 3 updates, got %d", c.updatedCount)
	}

	if c.lastConfig["version"] != 2 {
		t.Errorf("expected last config version 2, got %v", c.lastConfig["version"])
	}
}
