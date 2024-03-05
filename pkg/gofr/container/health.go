package container

import "context"

func (c *Container) Health(ctx context.Context) interface{} {
	datasources := make(map[string]interface{})

	if c.SQL != nil {
		datasources["sql"] = c.SQL.HealthCheck()
	}

	if c.Redis != nil {
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
