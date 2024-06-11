package container

import (
	"context"
	"reflect"
)

func (c *Container) Health(ctx context.Context) interface{} {
	healthMap := make(map[string]interface{})

	c.appHealth(healthMap)

	if !isNil(c.SQL) {
		healthMap["sql"] = c.SQL.HealthCheck()
	}

	if !isNil(c.Redis) {
		healthMap["redis"] = c.Redis.HealthCheck()
	}

	if c.PubSub != nil {
		healthMap["pubsub"] = c.PubSub.Health()
	}

	for name, svc := range c.Services {
		healthMap[name] = svc.HealthCheck(ctx)
	}

	return healthMap
}

func (c *Container) appHealth(healthMap map[string]interface{}) {
	healthMap["status"] = "UP"
	healthMap["name"] = c.GetAppName()
	healthMap["version"] = c.GetAppVersion()
}

func isNil(i interface{}) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
