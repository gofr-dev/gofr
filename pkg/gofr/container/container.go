package container

import (
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/service"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
)

// TODO - This can be a collection of interfaces instead of struct

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc which is shared across is placed here.
type Container struct {
	logging.Logger

	appName    string
	appVersion string

	Services       map[string]service.HTTP
	metricsManager metrics.Manager
	pubsub         pubsub.Client

	Redis *redis.Redis
	DB    *sql.DB
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		Logger:     logging.NewRemoteLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL")), conf.Get("REMOTE_LOG_URL")),
		appName:    conf.GetOrDefault("APP_NAME", "gofr-app"),
		appVersion: conf.GetOrDefault("APP_VERSION", "dev"),
	}

	c.Debug("Container is being created")

	c.Redis = redis.NewClient(conf, c.Logger)

	c.DB = sql.NewSQL(conf, c.Logger)

	c.metricsManager = metrics.NewMetricManager(exporters.Prometheus(c.appName, c.appVersion), c.Logger)

	c.pubsub = pubsub.New(conf, c.Logger)

	return c
}

// GetHTTPService returns registered http services.
// HTTP services are registered from AddHTTPService method of gofr object.
func (c *Container) GetHTTPService(serviceName string) service.HTTP {
	return c.Services[serviceName]
}

func (c *Container) Metrics() metrics.Manager {
	return c.metricsManager
}

func (c *Container) GetAppName() string {
	return c.appName
}

func (c *Container) GetAppVersion() string {
	return c.appVersion
}
