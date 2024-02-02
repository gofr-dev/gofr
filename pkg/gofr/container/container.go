package container

import (
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	logging.Logger
	Services map[string]service.HTTP
	Redis    *redis.Redis
	DB       *sql.DB
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		Logger: logging.NewLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL"))),
	}

	c.Debug("Container is being created")

	c.Redis = redis.NewClient(conf, c.Logger)

	c.DB = sql.NewSQL(conf, c.Logger)

	return c
}

// GetHTTPService returns registered http services.
// HTTP services are registered from AddHTTPService method of gofr object.
func (c *Container) GetHTTPService(serviceName string) service.HTTP {
	return c.Services[serviceName]
}
