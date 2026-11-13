package redis

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// poolMetricsInterval is how often the connection-pool statistics are sampled.
const poolMetricsInterval = 10 * time.Second

// Names of the pool gauges; the container registers them under the same names.
const (
	poolHitsMetric       = "app_redis_pool_hits"
	poolMissesMetric     = "app_redis_pool_misses"
	poolTimeoutsMetric   = "app_redis_pool_timeouts"
	poolTotalConnsMetric = "app_redis_pool_total_conns"
	poolIdleConnsMetric  = "app_redis_pool_idle_conns"
	poolStaleConnsMetric = "app_redis_pool_stale_conns"
)

// poolStatser is the part of the go-redis client that reports connection-pool statistics.
type poolStatser interface {
	PoolStats() *redis.PoolStats
}

// pushPoolMetrics samples the connection-pool statistics every interval and sets them as
// gauges, until stopSignal is closed. hits, misses and timeouts are cumulative since the
// client was created, so they are best read with rate()/increase(); the remaining gauges
// are point-in-time values.
func pushPoolMetrics(client poolStatser, metrics Metrics, hostname string, interval time.Duration, stopSignal <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopSignal:
			return
		case <-ticker.C:
			setPoolGauges(client.PoolStats(), metrics, hostname)
		}
	}
}

func setPoolGauges(stats *redis.PoolStats, metrics Metrics, hostname string) {
	if stats == nil {
		return
	}

	for name, value := range map[string]uint32{
		poolHitsMetric:       stats.Hits,
		poolMissesMetric:     stats.Misses,
		poolTimeoutsMetric:   stats.Timeouts,
		poolTotalConnsMetric: stats.TotalConns,
		poolIdleConnsMetric:  stats.IdleConns,
		poolStaleConnsMetric: stats.StaleConns,
	} {
		metrics.SetGauge(name, float64(value), "hostname", hostname)
	}
}
