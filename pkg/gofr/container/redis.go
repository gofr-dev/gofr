package container

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/extra/redisotel"
	"github.com/go-redis/redis/v8"
)

type redisConfig struct {
	HostName string
	Port     int
	Options  *redis.Options
}

// newRedisClient return a redis client if connection is successful based on Config.
// In case of error, it returns an error as second parameter.
func newRedisClient(config redisConfig) (*redis.Client, error) {
	if config.Options == nil {
		config.Options = new(redis.Options)
	}

	if config.Options.Addr == "" && config.HostName != "" && config.Port != 0 {
		config.Options.Addr = fmt.Sprintf("%s:%d", config.HostName, config.Port)
	}

	rc := redis.NewClient(config.Options)
	if err := rc.Ping(context.TODO()).Err(); err != nil {
		return nil, err
	}

	rc.AddHook(redisotel.TracingHook{})

	return rc, nil
}

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }
