package gofr

import (
	"database/sql"
	"strconv"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	Logger
	Redis *redis.Client
	DB    *sql.DB
}

func newContainer(config Config) *Container {
	c := &Container{
		Logger: newLogger(),
	}

	if config.Get("DISABLE_LOG") == "true" {
		c.Logger.Disable()
	}

	c.Log("Container is being created")

	// Connect Redis if REDIS_HOST is Set.
	if host := config.Get("REDIS_HOST"); host != "" {
		port, err := strconv.Atoi(config.Get("REDIS_PORT"))
		if err != nil {
			port = defaultRedisPort
		}

		c.Redis, err = NewRedisClient(RedisConfig{
			HostName: host,
			Port:     port,
		})

		if err != nil {
			c.Errorf("could not connect to redis at %s:%d\n error:", host, port, err)
		} else {
			c.Logf("connected to redis at %s:%d\n", host, port)
		}
	}

	if host := config.Get("DB_HOST"); host != "" {
		conf := DBConfig{
			HostName: host,
			User:     config.Get("DB_USER"),
			Password: config.Get("DB_PASSWORD"),
			Port:     config.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
			Database: config.Get("DB_NAME"),
		}
		db, err := NewMYSQL(&conf)
		c.DB = db

		if err != nil {
			c.Errorf("could not connect to database with config %v error: %v\n", conf, err)
		} else {
			c.Logf("connected to '%s' database at %s:%s\n", conf.Database, conf.HostName, conf.Port)
		}
	}

	return c
}
