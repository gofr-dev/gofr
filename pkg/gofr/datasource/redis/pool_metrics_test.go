package redis

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type fakePoolStatser struct {
	stats *redis.PoolStats
	calls atomic.Int32
}

func (f *fakePoolStatser) PoolStats() *redis.PoolStats {
	f.calls.Add(1)

	return f.stats
}

func TestPushPoolMetrics_SetsGaugesUntilStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	client := &fakePoolStatser{stats: &redis.PoolStats{
		Hits: 10, Misses: 3, Timeouts: 2, TotalConns: 5, IdleConns: 4, StaleConns: 1,
	}}

	sampled := make(chan struct{}, 1)

	for name, value := range map[string]float64{
		"app_redis_pool_hits":        10,
		"app_redis_pool_misses":      3,
		"app_redis_pool_timeouts":    2,
		"app_redis_pool_total_conns": 5,
		"app_redis_pool_idle_conns":  4,
		"app_redis_pool_stale_conns": 1,
	} {
		call := metrics.EXPECT().SetGauge(name, value, "hostname", "redis.example").MinTimes(1)

		if name == "app_redis_pool_stale_conns" {
			call.Do(func(string, float64, ...string) {
				select {
				case sampled <- struct{}{}:
				default:
				}
			})
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		pushPoolMetrics(client, metrics, "redis.example", time.Millisecond, stop)
	}()

	select {
	case <-sampled:
	case <-time.After(5 * time.Second):
		t.Fatal("pool gauges were never set")
	}

	close(stop)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pushPoolMetrics did not return after stop was closed")
	}
}

func TestSetPoolGauges_NilStats(t *testing.T) {
	// A nil snapshot must not panic or record anything: the strict mock has no expectations.
	setPoolGauges(nil, NewMockMetrics(gomock.NewController(t)), "redis.example")
}

func TestPushPoolMetrics_DoesNotSampleBeforeFirstInterval(t *testing.T) {
	client := &fakePoolStatser{stats: &redis.PoolStats{}}
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		pushPoolMetrics(client, NewMockMetrics(gomock.NewController(t)), "h", time.Hour, stop)
	}()

	close(stop)
	<-done

	assert.Zero(t, client.calls.Load())
}
