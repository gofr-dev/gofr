package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/datasource"
)

type Redis struct {
	*redis.Client
	logger datasource.Logger
}

type Config struct {
	HostName string
	Port     int
	Options  *redis.Options
}

// NewRedisClient return a redis client if connection is successful based on Config.
// In case of error, it returns an error as second parameter.
func NewRedisClient(config Config, logger datasource.Logger) (*Redis, error) {
	if config.Options == nil {
		config.Options = new(redis.Options)
	}

	if config.Options.Addr == "" && config.HostName != "" && config.Port != 0 {
		config.Options.Addr = fmt.Sprintf("%s:%d", config.HostName, config.Port)
	}

	rc := redis.NewClient(config.Options)
	rc.AddHook(&Redis{Client: rc, logger: logger})

	if err := rc.Ping(context.TODO()).Err(); err != nil {
		return nil, err
	}

	if err := redisotel.InstrumentTracing(rc); err != nil {
		panic(err)
	}

	// Enable metrics instrumentation.
	// if err := redisotel.InstrumentMetrics(rc); err != nil {
	//	panic(err)
	// }

	return &Redis{Client: rc, logger: logger}, nil
}

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }
