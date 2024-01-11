package container

import (
	"strconv"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logger"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
	"github.com/redis/go-redis/v9"
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	logger.Logger
	Redis *redis.Client
	DB    *datasource.DB
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		Logger: logger.NewLogger(logger.GetLevelFromString(conf.Get("LOG_LEVEL"))),
	}

	c.Logger.Debug("Container is being created")

	// Connect Redis if REDIS_HOST is Set.
	if host := conf.Get("REDIS_HOST"); host != "" {
		port, err := strconv.Atoi(conf.Get("REDIS_PORT"))
		if err != nil {
			port = defaultRedisPort
		}

		c.Redis, err = datasource.NewRedisClient(datasource.RedisConfig{
			HostName: host,
			Port:     port,
		})

		if err != nil {
			c.Logger.Errorf("could not connect to redis at %s:%d. error: %s", host, port, err)
		} else {
			c.Logger.Logf("connected to redis at %s:%d", host, port)
		}
	}

	if host := conf.Get("DB_HOST"); host != "" {
		conf := datasource.DBConfig{
			HostName: host,
			User:     conf.Get("DB_USER"),
			Password: conf.Get("DB_PASSWORD"),
			Port:     conf.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
			Database: conf.Get("DB_NAME"),
		}

		var err error

		c.DB, err = datasource.NewMYSQL(&conf, c.Logger)

		if err != nil {
			c.Logger.Errorf("could not connect with '%s' user to database '%s:%s'  error: %v",
				conf.User, conf.HostName, conf.Port, err)
		} else {
			c.Logger.Logf("connected to '%s' database at %s:%s", conf.Database, conf.HostName, conf.Port)
		}
	}

	return c
}
