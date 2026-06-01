package container

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"gofr.dev/pkg/gofr/datasource"
)

func (c *Container) Health(ctx context.Context) any {
	if cached, ok := c.healthCache.get(); ok {
		return cached
	}

	healthMap := make(map[string]any)
	var mu sync.Mutex
	var downCount atomic.Int64

	g, _ := errgroup.WithContext(ctx)

	if !isNil(c.SQL) {
		g.Go(func() error {
			health := c.SQL.HealthCheck()
			mu.Lock()
			healthMap["sql"] = health
			mu.Unlock()
			if health.Status == datasource.StatusDown {
				downCount.Add(1)
			}
			return nil
		})
	}

	if !isNil(c.Redis) {
		g.Go(func() error {
			health := c.Redis.HealthCheck()
			mu.Lock()
			healthMap["redis"] = health
			mu.Unlock()
			if health.Status == datasource.StatusDown {
				downCount.Add(1)
			}
			return nil
		})
	}

	if c.PubSub != nil {
		g.Go(func() error {
			health := c.PubSub.Health()
			mu.Lock()
			healthMap["pubsub"] = health
			mu.Unlock()
			if health.Status == datasource.StatusDown {
				downCount.Add(1)
			}
			return nil
		})
	}

	g.Go(func() error {
		count := checkExternalDBHealth(ctx, c, healthMap, &mu)
		downCount.Add(int64(count))
		return nil
	})

	g.Go(func() error {
		for name, svc := range c.Services {
			health := svc.HealthCheck(ctx)
			mu.Lock()
			healthMap[name] = health
			mu.Unlock()
			if health.Status == datasource.StatusDown {
				downCount.Add(1)
			}
		}
		return nil
	})

	_ = g.Wait()

	c.appHealth(healthMap, int(downCount.Load()))

	c.healthCache.set(healthMap)

	return healthMap
}

func checkExternalDBHealth(ctx context.Context, c *Container, healthMap map[string]any, mu *sync.Mutex) int {
	services := map[string]interface {
		HealthCheck(context.Context) (any, error)
	}{
		"mongo":         c.Mongo,
		"cassandra":     c.Cassandra,
		"clickHouse":    c.Clickhouse,
		"kv-store":      c.KVStore,
		"dgraph":        c.DGraph,
		"opentsdb":      c.OpenTSDB,
		"elasticsearch": c.Elasticsearch,
		"oracle":        c.Oracle,
		"couchbase":     c.Couchbase,
		"influx":        c.InfluxDB,
	}

	var (
		wg        sync.WaitGroup
		localMu   sync.Mutex
		downCount int
	)

	for name, service := range services {
		if !isNil(service) {
			wg.Add(1)

			go func(n string, s interface {
				HealthCheck(context.Context) (any, error)
			}) {
				defer wg.Done()

				health, err := s.HealthCheck(ctx)
				localMu.Lock()
				healthMap[n] = health
				if err != nil {
					downCount++
				}
				localMu.Unlock()
			}(name, service)
		}
	}

	wg.Wait()

	return downCount
}

func (c *Container) appHealth(healthMap map[string]any, downCount int) {
	healthMap["name"] = c.GetAppName()
	healthMap["version"] = c.GetAppVersion()

	if downCount == 0 {
		healthMap["status"] = datasource.StatusUp
	} else {
		healthMap["status"] = "DEGRADED"
	}
}

func isNil(i any) bool {
	val := reflect.ValueOf(i)

	return !val.IsValid() || val.IsNil()
}
