package container

import (
	"go.opentelemetry.io/otel/metric"

	"gofr.dev/pkg/gofr/config"
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

	Services       map[string]service.HTTP
	MetricsManager metrics.Manager
	exporter       metric.Meter

	Redis *redis.Redis
	DB    *sql.DB
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		Logger: logging.NewLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL"))),
	}

	c.Debug("Container is being created")

	c.Redis = redis.NewClient(conf, c.Logger)

	c.DB = sql.NewSQL(conf, c.Logger)

	if c.exporter == nil {
		c.exporter = exporters.OTLPStdOut(
			conf.GetOrDefault("APP_NAME", "gofr-app"),
			conf.GetOrDefault("APP_VERSION", "dev"))
	}

	c.MetricsManager = metrics.NewMetricManager(c.exporter)

	return c
}

func (c *Container) SetMetricsExporter(m metric.Meter) {
	c.exporter = m
}

// GetHTTPService returns registered http services.
// HTTP services are registered from AddHTTPService method of gofr object.
func (c *Container) GetHTTPService(serviceName string) service.HTTP {
	return c.Services[serviceName]
}

func (c *Container) UpdateMetric() metrics.Updater {
	return c.MetricsManager
}
