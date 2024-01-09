package container

import (
	"strconv"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	logging.Logger
	Redis *redis.Client
	DB    *DB
}

func NewContainer(cfg config.Config) *Container {
	c := &Container{
		Logger: logging.NewLogger(logging.GetLevelFromString(cfg.Get("LOG_LEVEL"))),
	}

	c.Debug("Container is being created")

	// Connect Redis if REDIS_HOST is Set.
	if host := cfg.Get("REDIS_HOST"); host != "" {
		port, err := strconv.Atoi(cfg.Get("REDIS_PORT"))
		if err != nil {
			port = defaultRedisPort
		}

		c.Redis, err = newRedisClient(redisConfig{
			HostName: host,
			Port:     port,
		})

		if err != nil {
			c.Errorf("could not connect to redis at %s:%d. error: %s", host, port, err)
		} else {
			c.Logf("connected to redis at %s:%d", host, port)
		}
	}

	if host := cfg.Get("DB_HOST"); host != "" {
		conf := dbConfig{
			HostName: host,
			User:     cfg.Get("DB_USER"),
			Password: cfg.Get("DB_PASSWORD"),
			Port:     cfg.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
			Database: cfg.Get("DB_NAME"),
		}
		db, err := newMYSQL(&conf)
		c.DB = &DB{db}

		if err != nil {
			c.Errorf("could not connect with '%s' user to database '%s:%s'  error: %v",
				conf.User, conf.HostName, conf.Port, err)
		} else {
			c.Logf("connected to '%s' database at %s:%s", conf.Database, conf.HostName, conf.Port)
		}
	}

	return c
}
