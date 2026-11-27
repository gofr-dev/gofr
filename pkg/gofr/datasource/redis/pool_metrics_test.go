package redis

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

const testPoolHost = "redis.example"

type fakePoolStatser struct {
	stats *redis.PoolStats
	calls atomic.Int32
}

func (f *fakePoolStatser) PoolStats() *redis.PoolStats {
	f.calls.Add(1)

	return f.stats
}

// fakePoolMetrics records what is exported. It has SetGauge, so it satisfies poolMetrics.
type fakePoolMetrics struct {
	mu       sync.Mutex
	counters map[string]int
	gauges   map[string]float64
	labels   map[string][]string
	gaugeSet chan struct{}

	gaugeCalls int
}

func newFakePoolMetrics() *fakePoolMetrics {
	return &fakePoolMetrics{
		counters: map[string]int{},
		gauges:   map[string]float64{},
		labels:   map[string][]string{},
		gaugeSet: make(chan struct{}, 1),
	}
}

func (*fakePoolMetrics) RecordHistogram(context.Context, string, float64, ...string) {}

func (f *fakePoolMetrics) IncrementCounter(_ context.Context, name string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.counters[name]++
	f.labels[name] = labels
}

func (f *fakePoolMetrics) SetGauge(name string, value float64, labels ...string) {
	f.mu.Lock()
	f.gauges[name] = value
	f.gaugeCalls++
	f.labels[name] = labels
	f.mu.Unlock()

	select {
	case f.gaugeSet <- struct{}{}:
	default:
	}
}

func (f *fakePoolMetrics) gauge(name string) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.gauges[name]
}

func TestPushPoolMetrics_ExportsStatsUntilStopped(t *testing.T) {
	client := &fakePoolStatser{stats: &redis.PoolStats{
		Hits: 10, Misses: 3, Timeouts: 2, TotalConns: 5, IdleConns: 4, StaleConns: 1,
	}}
	metrics := newFakePoolMetrics()
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		pushPoolMetrics(client, metrics, testPoolHost, time.Millisecond, stop)
	}()

	select {
	case <-metrics.gaugeSet:
	case <-time.After(5 * time.Second):
		t.Fatal("pool metrics were never exported")
	}

	close(stop)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pushPoolMetrics did not return after stop was closed")
	}

	// The stats never change, so however many samples ran the gauges equal the totals.
	assert.InDelta(t, 10, metrics.gauge(poolHitsMetric), 0)
	assert.InDelta(t, 3, metrics.gauge(poolMissesMetric), 0)
	assert.InDelta(t, 2, metrics.gauge(poolTimeoutsMetric), 0)
	assert.InDelta(t, 1, metrics.gauge(poolStaleConnsMetric), 0)
	assert.InDelta(t, 5, metrics.gauge(poolOpenConnsMetric), 0)
	assert.InDelta(t, 4, metrics.gauge(poolIdleConnsMetric), 0)
}

func TestPushPoolMetrics_DoesNotSampleBeforeFirstInterval(t *testing.T) {
	client := &fakePoolStatser{stats: &redis.PoolStats{}}
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		pushPoolMetrics(client, newFakePoolMetrics(), "h", time.Hour, stop)
	}()

	close(stop)
	<-done

	assert.Zero(t, client.calls.Load())
}

func TestRecordPoolStats_ExportsLatestValuesWithOneCallPerMetric(t *testing.T) {
	metrics := newFakePoolMetrics()

	recordPoolStats(&redis.PoolStats{Hits: 5, Misses: 2, Timeouts: 1, StaleConns: 1, TotalConns: 3, IdleConns: 2},
		metrics, testPoolHost)
	recordPoolStats(&redis.PoolStats{Hits: 9, Misses: 2, Timeouts: 4, StaleConns: 1, TotalConns: 6, IdleConns: 1},
		metrics, testPoolHost)

	// Gauges hold the latest value, not a running sum, and nothing is incremented.
	assert.InDelta(t, 9, metrics.gauge(poolHitsMetric), 0)
	assert.InDelta(t, 2, metrics.gauge(poolMissesMetric), 0)
	assert.InDelta(t, 4, metrics.gauge(poolTimeoutsMetric), 0)
	assert.InDelta(t, 1, metrics.gauge(poolStaleConnsMetric), 0)
	assert.InDelta(t, 6, metrics.gauge(poolOpenConnsMetric), 0)
	assert.InDelta(t, 1, metrics.gauge(poolIdleConnsMetric), 0)
	assert.Empty(t, metrics.counters)
	assert.Equal(t, 12, metrics.gaugeCalls)

	for _, name := range []string{poolHitsMetric, poolOpenConnsMetric, poolIdleConnsMetric} {
		assert.Equal(t, []string{"hostname", testPoolHost}, metrics.labels[name], name)
	}
}

func TestRecordPoolStats_LargeAndResetTotals(t *testing.T) {
	metrics := newFakePoolMetrics()

	recordPoolStats(&redis.PoolStats{Hits: ^uint32(0)}, metrics, testPoolHost)
	assert.InDelta(t, float64(^uint32(0)), metrics.gauge(poolHitsMetric), 0)

	// A pool reset simply lowers the gauge; there is no delta to underflow.
	recordPoolStats(&redis.PoolStats{Hits: 2}, metrics, testPoolHost)
	assert.InDelta(t, 2, metrics.gauge(poolHitsMetric), 0)
}

func TestRecordPoolStats_NilStats(t *testing.T) {
	metrics := newFakePoolMetrics()

	recordPoolStats(nil, metrics, testPoolHost)

	assert.Empty(t, metrics.counters)
	assert.Empty(t, metrics.gauges)
}

func TestNewClient_PoolMetricsSampler(t *testing.T) {
	shortenPoolMetricsInterval(t)

	t.Run("starts for metrics with SetGauge and Close stops it", func(t *testing.T) {
		metrics := newFakePoolMetrics()
		r := newClientForPoolTest(t, metrics)

		select {
		case <-metrics.gaugeSet:
		case <-time.After(5 * time.Second):
			t.Fatal("sampler did not export pool metrics")
		}

		require.NoError(t, r.Close())

		// Let a sample that was already in flight finish, then expect no more.
		time.Sleep(20 * time.Millisecond)

		for len(metrics.gaugeSet) > 0 {
			<-metrics.gaugeSet
		}

		time.Sleep(50 * time.Millisecond)

		assert.Empty(t, metrics.gaugeSet, "sampler kept running after Close")
	})

	t.Run("does not start for metrics without SetGauge", func(t *testing.T) {
		metrics := &twoMethodMetrics{}
		r := newClientForPoolTest(t, metrics)

		// The ping alone makes go-redis count pool hits or misses, so a running sampler would have counted them by now.
		time.Sleep(50 * time.Millisecond)

		require.NoError(t, r.Close())

		assert.Zero(t, metrics.poolCounters.Load())
	})
}

func shortenPoolMetricsInterval(t *testing.T) {
	t.Helper()

	original := poolMetricsInterval
	poolMetricsInterval = time.Millisecond

	t.Cleanup(func() { poolMetricsInterval = original })
}

func newClientForPoolTest(t *testing.T, metrics Metrics) *Redis {
	t.Helper()

	s, err := miniredis.Run()
	require.NoError(t, err)

	t.Cleanup(s.Close)

	r := NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": s.Host(), "REDIS_PORT": s.Port()}),
		logging.NewMockLogger(logging.ERROR), metrics)
	require.NotNil(t, r)

	return r
}

// twoMethodMetrics implements only the released Metrics interface and counts pool counter increments.
type twoMethodMetrics struct {
	poolCounters atomic.Int32
}

func (*twoMethodMetrics) RecordHistogram(context.Context, string, float64, ...string) {}

func (m *twoMethodMetrics) IncrementCounter(_ context.Context, name string, _ ...string) {
	if strings.HasPrefix(name, "app_redis_pool_") {
		m.poolCounters.Add(1)
	}
}
