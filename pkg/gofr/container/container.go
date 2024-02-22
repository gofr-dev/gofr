package container

import (
	"strconv"
	"strings"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/google"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
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
		Logger: logging.NewRemoteLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL")), conf.Get("REMOTE_LOG_URL"),
			conf.GetOrDefault("REMOTE_LOG_FETCH_INTERVAL", "15")),
		appName:    conf.GetOrDefault("APP_NAME", "gofr-app"),
		appVersion: conf.GetOrDefault("APP_VERSION", "dev"),
	}

	c.Debug("Container is being created")

	c.metricsManager = metrics.NewMetricsManager(exporters.Prometheus(c.appName, c.appVersion), c.Logger)

	// Register framework metrics
	c.registerFrameworkMetrics()

	c.Redis = redis.NewClient(conf, c.Logger, c.metricsManager)

	c.DB = sql.NewSQL(conf, c.Logger, c.metricsManager)

	switch strings.ToUpper(conf.Get("PUBSUB_BACKEND")) {
	case "KAFKA":
		if conf.Get("PUBSUB_BROKER") != "" {
			partition, _ := strconv.Atoi(conf.GetOrDefault("PARTITION_SIZE", "0"))
			offSet, _ := strconv.Atoi(conf.GetOrDefault("PUBSUB_OFFSET", "-1"))

			c.pubsub = kafka.New(kafka.Config{
				Broker:          conf.Get("PUBSUB_HOST"),
				Partition:       partition,
				ConsumerGroupID: conf.Get("CONSUMER_ID"),
				Topic:           conf.Get("PUBSUB_TOPIC"),
				OffSet:          offSet,
			}, c.Logger)
		}
	case "GOOGLE":
		c.pubsub = google.New(google.Config{
			ProjectID:        conf.Get("GOOGLE_PROJECT_ID"),
			SubscriptionName: conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
		}, c.Logger)
	}

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

func (c *Container) registerFrameworkMetrics() {
	// system info metrics
	c.Metrics().NewGauge("app_go_routines", "Number of Go routines running.")
	c.Metrics().NewGauge("app_sys_memory_alloc", "Number of bytes allocated for heap objects.")
	c.Metrics().NewGauge("app_sys_total_alloc", "Number of cumulative bytes allocated for heap objects.")
	c.Metrics().NewGauge("app_go_numGC", "Number of completed Garbage Collector cycles.")
	c.Metrics().NewGauge("app_go_sys", "Number of total bytes of memory.")

	histogramBuckets := []float64{.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30}

	// http metrics
	c.Metrics().NewHistogram("app_http_response", "Response time of http requests in seconds.", histogramBuckets...)
	c.Metrics().NewHistogram("app_http_service_response", "Response time of http service requests in seconds.", histogramBuckets...)

	// redis metrics
	c.Metrics().NewHistogram("app_redis_stats", "Observes the response time for Redis commands.", histogramBuckets...)

	// sql metrics
	c.Metrics().NewHistogram("app_sql_stats", "Observes the response time for SQL queries.", histogramBuckets...)
	c.Metrics().NewGauge("app_sql_open_connections", "Number of open SQL connections.")
	c.Metrics().NewGauge("app_sql_inUse_connections", "Number of inUse SQL connections.")
}

func (c *Container) GetAppName() string {
	return c.appName
}

func (c *Container) GetAppVersion() string {
	return c.appVersion
}

func (c *Container) GetPublisher() pubsub.Publisher {
	return c.pubsub
}

func (c *Container) GetSubscriber() pubsub.Subscriber {
	return c.pubsub
}
