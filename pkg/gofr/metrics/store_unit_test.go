package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestStore_SetAndGetCounter(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	counter, err := meter.Int64Counter("test-counter")
	require.NoError(t, err)

	err = store.setCounter("test-counter", counter)
	require.NoError(t, err)

	got, err := store.getCounter("test-counter")
	require.NoError(t, err)
	require.Equal(t, counter, got)
}

func TestStore_SetAndGetUpDownCounter(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	udc, err := meter.Float64UpDownCounter("test-updown")
	require.NoError(t, err)

	err = store.setUpDownCounter("test-updown", udc)
	require.NoError(t, err)

	got, err := store.getUpDownCounter("test-updown")
	require.NoError(t, err)
	require.Equal(t, udc, got)
}

func TestStore_SetAndGetHistogram(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	hist, err := meter.Float64Histogram("test-hist")
	require.NoError(t, err)

	err = store.setHistogram("test-hist", hist)
	require.NoError(t, err)

	got, err := store.getHistogram("test-hist")
	require.NoError(t, err)
	require.Equal(t, hist, got)
}

func TestStore_SetAndGetGauge(t *testing.T) {
	store := newOtelStore()

	g := &float64Gauge{}

	err := store.setGauge("test-gauge", g)
	require.NoError(t, err)

	got, err := store.getGauge("test-gauge")
	require.NoError(t, err)
	require.Equal(t, g, got)
}

func TestStore_DuplicateMetricRegistration(t *testing.T) {
	store := newOtelStore()
	meter := noop.NewMeterProvider().Meter("test")

	counter, _ := meter.Int64Counter("dup-counter")
	_ = store.setCounter("dup-counter", counter)
	err := store.setCounter("dup-counter", counter)

	require.ErrorContains(t, err, "already registered")
}

func TestStore_GetNonExistentMetric(t *testing.T) {
	store := newOtelStore()

	_, err := store.getCounter("no-counter")
	require.ErrorContains(t, err, "not registered")

	_, err = store.getUpDownCounter("no-updown")
	require.ErrorContains(t, err, "not registered")

	_, err = store.getHistogram("no-hist")
	require.ErrorContains(t, err, "not registered")

	_, err = store.getGauge("no-gauge")
	require.ErrorContains(t, err, "not registered")
}

func TestStore_ConcurrentGaugeSetGet(t *testing.T) {
	store := newOtelStore()
	g := &float64Gauge{
		observations: make(map[attribute.Set]float64),
	}
	err := store.setGauge("concurrent-gauge", g)
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.NotNil(t, got)
}
