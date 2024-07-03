package container

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func (c *Container) Health(ctx context.Context) interface{} {
	var (
		healthMap = make(map[string]interface{})
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

	if !isNil(c.Mongo) {
		health := c.Mongo.HealthCheck()
		if strings.Contains(fmt.Sprint(health), "DOWN") {
			downCount++
		}

		healthMap["mongo"] = health
	}

	if !isNil(c.Cassandra) {
		health := c.Cassandra.HealthCheck()
		if strings.Contains(fmt.Sprint(health), "DOWN") {
			downCount++
		}

		healthMap["cassandra"] = health
	}

	if !isNil(c.Clickhouse) {
		health := c.Clickhouse.HealthCheck()
		if strings.Contains(fmt.Sprint(health), "DOWN") {
			downCount++
		}

		healthMap["clickHouse"] = health
	}

	if c.PubSub != nil {
		health := c.PubSub.Health()
		if health.Status == statusDown {
			downCount++
		}

		healthMap["pubsub"] = health
	}

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

func (c *Container) appHealth(healthMap map[string]interface{}, downCount int) {
	healthMap["name"] = c.GetAppName()
	healthMap["version"] = c.GetAppVersion()

	if downCount == 0 {
		healthMap["status"] = "UP"
	} else {
		healthMap["status"] = "DEGRADED"
	}
}

func isNil(i interface{}) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
