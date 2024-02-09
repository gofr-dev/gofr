package container

import "context"

func (c *Container) Health(ctx context.Context) interface{} {
	datasources := make(map[string]interface{})

	if c.DB != nil {
		datasources["sql"] = c.DB.HealthCheck()
	}

	if c.Redis != nil {
		datasources["redis"] = c.Redis.HealthCheck()
	}

	for name, svc := range c.Services {
		datasources[name] = svc.HealthCheck(ctx)
	}

	return datasources
}
