package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeManager struct {
	calls []string
}

func (f *fakeManager) NewCounter(name, _ string) { f.calls = append(f.calls, "NewCounter:"+name) }
func (f *fakeManager) NewUpDownCounter(name, _ string) {
	f.calls = append(f.calls, "NewUpDownCounter:"+name)
}
func (f *fakeManager) NewHistogram(name, _ string, _ ...float64) {
	f.calls = append(f.calls, "NewHistogram:"+name)
}
func (f *fakeManager) NewGauge(name, _ string) { f.calls = append(f.calls, "NewGauge:"+name) }

func (f *fakeManager) IncrementCounter(_ context.Context, name string, _ ...string) {
	f.calls = append(f.calls, "IncrementCounter:"+name)
}

func (f *fakeManager) DeltaUpDownCounter(_ context.Context, name string, _ float64, _ ...string) {
	f.calls = append(f.calls, "DeltaUpDownCounter:"+name)
}

func (f *fakeManager) RecordHistogram(_ context.Context, name string, _ float64, _ ...string) {
	f.calls = append(f.calls, "RecordHistogram:"+name)
}

func (f *fakeManager) SetGauge(name string, _ float64, _ ...string) {
	f.calls = append(f.calls, "SetGauge:"+name)
}

func TestGlobal_DefaultsToNoop(t *testing.T) {
	// No SetGlobal call in this test — Global() must return a usable no-op,
	// not nil, so callers never need a nil check.
	assert.NotPanics(t, func() {
		IncrementCounter(t.Context(), "some_counter")
		RecordHistogram(t.Context(), "some_histogram", 1.0)
		SetGauge("some_gauge", 1.0)
	})
}

func TestGlobal_SetGlobalRedirectsPackageLevelHelpers(t *testing.T) {
	original := Global()
	defer SetGlobal(original)

	f := &fakeManager{}
	SetGlobal(f)

	NewCounter("c", "desc")
	NewUpDownCounter("udc", "desc")
	NewHistogram("h", "desc", 1, 2, 3)
	NewGauge("g", "desc")
	IncrementCounter(t.Context(), "c")
	DeltaUpDownCounter(t.Context(), "udc", 1)
	RecordHistogram(t.Context(), "h", 1)
	SetGauge("g", 1)

	assert.Equal(t, []string{
		"NewCounter:c",
		"NewUpDownCounter:udc",
		"NewHistogram:h",
		"NewGauge:g",
		"IncrementCounter:c",
		"DeltaUpDownCounter:udc",
		"RecordHistogram:h",
		"SetGauge:g",
	}, f.calls)
}
