package config

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockConfigSubscriber struct {
	mu           sync.Mutex
	updatedCount int
	lastConfig   map[string]any
}

func (m *mockConfigSubscriber) UpdateConfig(cfg map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updatedCount++
	m.lastConfig = cfg
}

type mockConfigProvider struct {
	mu           sync.Mutex
	registered   []Subscriber
	started      bool
	startTrigger chan map[string]any
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		startTrigger: make(chan map[string]any, 1),
	}
}

func (m *mockConfigProvider) Register(c Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registered = append(m.registered, c)
}

func (m *mockConfigProvider) Start() {
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

func (m *mockConfigProvider) pushConfig(cfg map[string]any) {
	m.startTrigger <- cfg
}

func TestConfigProvider_RegisterAndStart(t *testing.T) {
	rc := newMockConfigProvider()
	c1 := &mockConfigSubscriber{}
	c2 := &mockConfigSubscriber{}

	rc.Register(c1)
	rc.Register(c2)

	rc.Start()

	rc.pushConfig(map[string]any{"log_level": "DEBUG"})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, c1.updatedCount, "c1 update count")
	assert.Equal(t, 1, c2.updatedCount, "c2 update count")
	assert.Equal(t, "DEBUG", c1.lastConfig["log_level"], "c1 config value")
	assert.True(t, rc.started, "provider started")
}

func TestConfigProvider_NoRegisteredComponents(_ *testing.T) {
	rc := newMockConfigProvider()
	rc.Start()

	rc.pushConfig(map[string]any{"feature": "enabled"})
	time.Sleep(20 * time.Millisecond)
}

func TestConfigSubscriber_UpdateConfigMultipleTimes(t *testing.T) {
	rc := newMockConfigProvider()
	c := &mockConfigSubscriber{}

	rc.Register(c)
	rc.Start()

	expectedLast := 0

	for i := range 3 {
		rc.pushConfig(map[string]any{"version": i})
		expectedLast = i
	}

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 3, c.updatedCount, "update count")

	version, ok := c.lastConfig["version"].(int)
	assert.True(t, ok)
	assert.Equal(t, expectedLast, version, "last version")
}
