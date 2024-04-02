package container

import "context"

func (c *Container) Health(ctx context.Context) interface{} {
	datasources := make(map[string]interface{})

	switch c.SQL.(type) {
	case nil:
	default:
		datasources["sql"] = c.SQL.HealthCheck()
	}

	switch c.Redis.(type) {
	case nil:
	default:
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
