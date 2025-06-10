package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestStore_SetAndGetCounter(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	counter, err := meter.Int64Counter("test-counter")
	assert.NoError(t, err)

	err = store.setCounter("test-counter", counter)
	assert.NoError(t, err)

	got, err := store.getCounter("test-counter")
	assert.NoError(t, err)
	assert.Equal(t, counter, got)
}

func TestStore_SetAndGetUpDownCounter(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	udc, err := meter.Float64UpDownCounter("test-updown")
	assert.NoError(t, err)

	err = store.setUpDownCounter("test-updown", udc)
	assert.NoError(t, err)

	got, err := store.getUpDownCounter("test-updown")
	assert.NoError(t, err)
	assert.Equal(t, udc, got)
}

func TestStore_SetAndGetHistogram(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	hist, err := meter.Float64Histogram("test-hist")
	assert.NoError(t, err)

	err = store.setHistogram("test-hist", hist)
	assert.NoError(t, err)

	got, err := store.getHistogram("test-hist")
	assert.NoError(t, err)
	assert.Equal(t, hist, got)
}

func TestStore_SetAndGetGauge(t *testing.T) {
	store := newOtelStore()

	g := &float64Gauge{}

	err := store.setGauge("test-gauge", g)
	assert.NoError(t, err)

	got, err := store.getGauge("test-gauge")
	assert.NoError(t, err)
	assert.Equal(t, g, got)
}

func TestStore_DuplicateMetricRegistration(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	counter, _ := meter.Int64Counter("dup-counter")
	_ = store.setCounter("dup-counter", counter)
	err := store.setCounter("dup-counter", counter)

	assert.ErrorContains(t, err, "already registered")
}

func TestStore_GetNonExistentMetric(t *testing.T) {
	store := newOtelStore()

	_, err := store.getCounter("no-counter")
	assert.ErrorContains(t, err, "not registered")

	_, err = store.getUpDownCounter("no-updown")
	assert.ErrorContains(t, err, "not registered")

	_, err = store.getHistogram("no-hist")
	assert.ErrorContains(t, err, "not registered")

	_, err = store.getGauge("no-gauge")
	assert.ErrorContains(t, err, "not registered")
}

func TestStore_ConcurrentGaugeSetGet(t *testing.T) {
	store := newOtelStore()
	g := &float64Gauge{
		observations: make(map[attribute.Set]float64),
	}
	store.setGauge("concurrent-gauge", g)

	var wg sync.WaitGroup
	for i := range make([]struct{}, 10) {
		wg.Add(1)
		go func(val float64) {
			defer wg.Done()
			g.mu.Lock()
			g.observations[attribute.NewSet()] = val
			g.mu.Unlock()
		}(float64(i))
	}
	wg.Wait()

	got, err := store.getGauge("concurrent-gauge")
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
