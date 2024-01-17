package container

import (
	"strconv"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
	"gofr.dev/pkg/gofr/config"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	logging.Logger
	Redis *gofrRedis.Redis
	DB    *sql.DB
}

func (c *Container) Health() interface{} {
	datasources := make(map[string]interface{})

	//datasources["sql"] = c.DB.HealthCheck()
	datasources["redis"] = c.Redis.HealthCheck()

	return datasources
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		Logger: logging.NewLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL"))),
	}

	c.Debug("Container is being created")

	// Connect Redis if REDIS_HOST is Set.
	if host := conf.Get("REDIS_HOST"); host != "" {
		port, err := strconv.Atoi(conf.Get("REDIS_PORT"))
		if err != nil {
			port = defaultRedisPort
		}

		c.Redis, err = gofrRedis.NewRedisClient(gofrRedis.Config{
			HostName: host,
			Port:     port,
			Options:  nil,
		}, c.Logger)

		if err != nil {
			c.Errorf("could not connect to redis at %s:%d. error: %s", host, port, err)
		} else {
			c.Logf("connected to redis at %s:%d", host, port)
		}

	}

	if host := conf.Get("DB_HOST"); host != "" {
		conf := sql.DBConfig{
			HostName: host,
			User:     conf.Get("DB_USER"),
			Password: conf.Get("DB_PASSWORD"),
			Port:     conf.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
			Database: conf.Get("DB_NAME"),
		}

		var err error

		c.DB, err = sql.NewMYSQL(&conf, c.Logger)

		if err != nil {
			c.Errorf("could not connect with '%s' user to database '%s:%s'  error: %v",
				conf.User, conf.HostName, conf.Port, err)
		} else {
			c.Logf("connected to '%s' database at %s:%s", conf.Database, conf.HostName, conf.Port)
		}
	}

	return c
}
