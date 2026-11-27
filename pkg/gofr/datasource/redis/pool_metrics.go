package redis

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// poolMetricsInterval is how often the connection-pool statistics are sampled.
//
//nolint:gochecknoglobals // a variable only so tests can shorten the interval.
var poolMetricsInterval = 10 * time.Second

// Names of the pool metrics; the container registers them under the same names.
const (
	poolHitsMetric        = "app_redis_pool_hits"
	poolMissesMetric      = "app_redis_pool_misses"
	poolTimeoutsMetric    = "app_redis_pool_timeouts"
	poolStaleConnsMetric  = "app_redis_pool_stale_conns"
	poolOpenConnsMetric   = "app_redis_open_connections"
	poolIdleConnsMetric   = "app_redis_idle_connections"
	poolHostnameLabelName = "hostname"
)

// poolMetrics is the metrics recorder needed to export pool statistics. It is kept separate from
// Metrics so that existing Metrics implementations keep compiling; the sampler only runs for
// recorders that also support gauges.
type poolMetrics interface {
	Metrics
	SetGauge(name string, value float64, labels ...string)
}

// poolStatser is the part of the go-redis client that reports connection-pool statistics.
type poolStatser interface {
	PoolStats() *redis.PoolStats
}

// pushPoolMetrics samples the connection-pool statistics every interval until stopSignal is closed.
// Every statistic is exported as a gauge with one call per metric per sample. Hits, misses, timeouts
// and stale connections are cumulative totals since the client started; graph them with rate().
func pushPoolMetrics(client poolStatser, metrics poolMetrics, hostname string, interval time.Duration, stopSignal <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopSignal:
			return
		case <-ticker.C:
			recordPoolStats(client.PoolStats(), metrics, hostname)
		}
	}
}

// recordPoolStats exports stats. A nil snapshot is ignored.
func recordPoolStats(stats *redis.PoolStats, metrics poolMetrics, hostname string) {
	if stats == nil {
		return
	}

	labels := []string{poolHostnameLabelName, hostname}

	metrics.SetGauge(poolHitsMetric, float64(stats.Hits), labels...)
	metrics.SetGauge(poolMissesMetric, float64(stats.Misses), labels...)
	metrics.SetGauge(poolTimeoutsMetric, float64(stats.Timeouts), labels...)
	metrics.SetGauge(poolStaleConnsMetric, float64(stats.StaleConns), labels...)
	metrics.SetGauge(poolOpenConnsMetric, float64(stats.TotalConns), labels...)
	metrics.SetGauge(poolIdleConnsMetric, float64(stats.IdleConns), labels...)
}
