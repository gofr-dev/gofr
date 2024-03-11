package container

import (
	"strconv"
	"strings"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/google"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
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
	PubSub         pubsub.Client

	Redis *redis.Redis
	SQL   *sql.DB
}

func NewEmptyContainer() *Container {
	return &Container{}
}

func NewContainer(conf config.Config) *Container {
	c := &Container{
		appName:    conf.GetOrDefault("APP_NAME", "gofr-app"),
		appVersion: conf.GetOrDefault("APP_VERSION", "dev"),
	}

	c.Create(conf)

	return c
}

func (c *Container) Create(conf config.Config) {
	if c.appName != "" {
		c.appName = conf.GetOrDefault("APP_NAME", "gofr-app")
	}

	if c.appVersion != "" {
		c.appVersion = conf.GetOrDefault("APP_VERSION", "dev")
	}

	if c.Logger == nil {
		c.Logger = logging.NewRemoteLogger(logging.GetLevelFromString(conf.Get("LOG_LEVEL")), conf.Get("REMOTE_LOG_URL"),
			conf.GetOrDefault("REMOTE_LOG_FETCH_INTERVAL", "15"))
	}

	c.Debug("Container is being created")

	c.metricsManager = metrics.NewMetricsManager(exporters.Prometheus(c.appName, c.appVersion), c.Logger)

	// Register framework metrics
	c.registerFrameworkMetrics()

	c.Redis = redis.NewClient(conf, c.Logger, c.metricsManager)

	c.SQL = sql.NewSQL(conf, c.Logger, c.metricsManager)

	switch strings.ToUpper(conf.Get("PUBSUB_BACKEND")) {
	case "KAFKA":
		if conf.Get("PUBSUB_BROKER") != "" {
			partition, _ := strconv.Atoi(conf.GetOrDefault("PARTITION_SIZE", "0"))
			offSet, _ := strconv.Atoi(conf.GetOrDefault("PUBSUB_OFFSET", "-1"))

			c.PubSub = kafka.New(kafka.Config{
				Broker:          conf.Get("PUBSUB_BROKER"),
				Partition:       partition,
				ConsumerGroupID: conf.Get("CONSUMER_ID"),
				OffSet:          offSet,
			}, c.Logger, c.metricsManager)
		}
	case "GOOGLE":
		c.PubSub = google.New(google.Config{
			ProjectID:        conf.Get("GOOGLE_PROJECT_ID"),
			SubscriptionName: conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
		}, c.Logger, c.metricsManager)
	case "MQTT":
		var qos byte

		port, _ := strconv.Atoi(conf.Get("MQTT_PORT"))
		order, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_MESSAGE_ORDER", "false"))

		switch conf.Get("MQTT_QOS") {
		case "1":
			qos = 1
		case "2":
			qos = 2
		default:
			qos = 0
		}

		configs := &mqtt.Config{
			Protocol: conf.GetOrDefault("MQTT_PROTOCOL", "tcp"), // using tcp as default method to connect to broker
			Hostname: conf.Get("MQTT_HOST"),
			Port:     port,
			Username: conf.Get("MQTT_USER"),
			Password: conf.Get("MQTT_PASSWORD"),
			ClientID: conf.Get("MQTT_CLIENT_ID_SUFFIX"),
			QoS:      qos,
			Order:    order,
		}

		c.PubSub = mqtt.New(configs, c.Logger, c.metricsManager)
	}
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

	// http metrics
	httpBuckets := []float64{.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30}
	c.Metrics().NewHistogram("app_http_response", "Response time of http requests in seconds.", httpBuckets...)
	c.Metrics().NewHistogram("app_http_service_response", "Response time of http service requests in seconds.", httpBuckets...)

	// redis metrics
	redisBuckets := []float64{50, 75, 100, 125, 150, 200, 300, 500, 750, 1000, 1250, 1500, 2000, 2500, 3000}
	c.Metrics().NewHistogram("app_redis_stats", "Response time of Redis commands in microseconds.", redisBuckets...)

	// sql metrics
	sqlBuckets := []float64{50, 75, 100, 125, 150, 200, 300, 500, 750, 1000, 2000, 3000, 4000, 5000, 7500, 10000}
	c.Metrics().NewHistogram("app_sql_stats", "Response time of SQL queries in microseconds.", sqlBuckets...)
	c.Metrics().NewGauge("app_sql_open_connections", "Number of open SQL connections.")
	c.Metrics().NewGauge("app_sql_inUse_connections", "Number of inUse SQL connections.")

	// pubsub metrics
	c.Metrics().NewCounter("app_pubsub_publish_total_count", "Number of total publish operations.")
	c.Metrics().NewCounter("app_pubsub_publish_success_count", "Number of successful publish operations.")
	c.Metrics().NewCounter("app_pubsub_subscribe_total_count", "Number of total subscribe operations.")
	c.Metrics().NewCounter("app_pubsub_subscribe_success_count", "Number of successful subscribe operations.")
}

func (c *Container) GetAppName() string {
	return c.appName
}

func (c *Container) GetAppVersion() string {
	return c.appVersion
}

func (c *Container) GetPublisher() pubsub.Publisher {
	return c.PubSub
}

func (c *Container) GetSubscriber() pubsub.Subscriber {
	return c.PubSub
}
