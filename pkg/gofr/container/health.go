package container

import (
	"context"
	"reflect"
)

func (c *Container) Health(ctx context.Context) interface{} {
	datasources := make(map[string]interface{})

	if !isNil(c.SQL) {
		datasources["sql"] = c.SQL.HealthCheck()
	}

	if !isNil(c.Redis) {
		datasources["redis"] = c.Redis.HealthCheck()
	}

	if c.PubSub != nil {
		datasources["pubsub"] = c.PubSub.Health()
	}

	for name, svc := range c.Services {
		datasources[name] = svc.HealthCheck(ctx)
	}

	return datasources
}

func isNil(i interface{}) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}
