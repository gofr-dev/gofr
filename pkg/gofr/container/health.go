package container

import (
	"context"
	"reflect"
)

func (c *Container) Health(ctx context.Context) any {
	var (
		healthMap = make(map[string]any)
		downCount int
	)

	const statusDown = "DOWN"

	if !isNil(c.SQL) {
		health := c.SQL.HealthCheck()
		if health.Status == statusDown {
			downCount++
		}

		healthMap["sql"] = health
	}

	if !isNil(c.Redis) {
		health := c.Redis.HealthCheck()
		if health.Status == statusDown {
			downCount++
		}

		healthMap["redis"] = health
	}

	if c.PubSub != nil {
		health := c.PubSub.Health()
		if health.Status == statusDown {
			downCount++
		}

		healthMap["pubsub"] = health
	}

	downCount += checkExternalDBHealth(ctx, c, healthMap)

	for name, svc := range c.Services {
		health := svc.HealthCheck(ctx)
		if health.Status == statusDown {
			downCount++
		}

		healthMap[name] = health
	}

	c.appHealth(healthMap, downCount)

	return healthMap
}

func checkExternalDBHealth(ctx context.Context, c *Container, healthMap map[string]any) (downCount int) {
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
	}

	for name, service := range services {
		if !isNil(service) {
			health, err := service.HealthCheck(ctx)
			if err != nil {
				downCount++
			}

			healthMap[name] = health
		}
	}

	return downCount
}

func (c *Container) appHealth(healthMap map[string]any, downCount int) {
	healthMap["name"] = c.GetAppName()
	healthMap["version"] = c.GetAppVersion()

	if downCount == 0 {
		healthMap["status"] = "UP"
	} else {
		healthMap["status"] = "DEGRADED"
	}
}

func isNil(i any) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
