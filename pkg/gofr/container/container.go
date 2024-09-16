package container

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
	"gofr.dev/pkg/gofr/datasource/pubsub/nats"
	"gofr.dev/pkg/gofr/websocket"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/google"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/logging/remotelogger"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/version"
)

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc. which is shared across is placed here.
type Container struct {
	logging.Logger

	appName    string
	appVersion string

	Services       map[string]service.HTTP
	metricsManager metrics.Manager
	PubSub         pubsub.Client

	Redis Redis
	SQL   DB

	Cassandra  Cassandra
	Clickhouse Clickhouse
	Mongo      Mongo

	KVStore KVStore

	File file.FileSystem
}

func NewContainer(conf config.Config) *Container {
	if conf == nil {
		return &Container{}
	}

	c := &Container{
		appName:    conf.GetOrDefault("APP_NAME", "gofr-app"),
		appVersion: conf.GetOrDefault("APP_VERSION", "dev"),
	}

	c.Create(conf)

	return c
}

func (c *Container) Create(conf config.Config) {
	c.initializeAppInfo(conf)
	c.initializeLogger(conf)
	c.initializeMetrics()
	c.initializeRedisAndSQL(conf)
	c.initializePubSub(conf)
	c.initializeFile()
}

func (c *Container) initializeAppInfo(conf config.Config) {
	if c.appName == "" {
		c.appName = conf.GetOrDefault("APP_NAME", "gofr-app")
	}

	if c.appVersion == "" {
		c.appVersion = conf.GetOrDefault("APP_VERSION", "dev")
	}
}

func (c *Container) initializeLogger(conf config.Config) {
	if c.Logger != nil {
		return
	}

	levelFetchConfig := c.getLevelFetchConfig(conf)
	c.Logger = remotelogger.New(
		logging.GetLevelFromString(conf.Get("LOG_LEVEL")),
		conf.Get("REMOTE_LOG_URL"),
		time.Duration(levelFetchConfig)*time.Second,
	)
	c.Debug("Container is being created")
}

func (c *Container) getLevelFetchConfig(conf config.Config) int {
	levelFetchConfig, err := strconv.Atoi(conf.GetOrDefault("REMOTE_LOG_FETCH_INTERVAL", "15"))
	if err != nil {
		levelFetchConfig = 15

		c.Logger.Error("invalid value for REMOTE_LOG_FETCH_INTERVAL. setting default of 15 sec.")
	}

	return levelFetchConfig
}

func (c *Container) initializeMetrics() {
	c.metricsManager = metrics.NewMetricsManager(exporters.Prometheus(c.GetAppName(), c.GetAppVersion()), c.Logger)
	c.registerFrameworkMetrics()
	c.Metrics().SetGauge("app_info", 1,
		"app_name", c.GetAppName(), "app_version", c.GetAppVersion(), "framework_version", version.Framework)
}

func (c *Container) initializeRedisAndSQL(conf config.Config) {
	c.Redis = redis.NewClient(conf, c.Logger, c.metricsManager)
	c.SQL = sql.NewSQL(conf, c.Logger, c.metricsManager)
}

func (c *Container) initializePubSub(conf config.Config) {
	switch strings.ToUpper(conf.Get("PUBSUB_BACKEND")) {
	case "KAFKA":
		c.initializeKafka(conf)
	case "GOOGLE":
		c.initializeGoogle(conf)
	case "MQTT":
		c.PubSub = c.createMqttPubSub(conf)
	case "NATS":
		c.initializeNATS(conf)
	}
}

func (c *Container) initializeKafka(conf config.Config) {
	if conf.Get("PUBSUB_BROKER") == "" {
		return
	}

	c.PubSub = kafka.New(c.getKafkaConfig(conf), c.Logger, c.metricsManager)
}

func (c *Container) getKafkaConfig(conf config.Config) kafka.Config {
	return kafka.Config{
		Broker:          conf.Get("PUBSUB_BROKER"),
		Partition:       c.getIntConfig(conf, "PARTITION_SIZE", 0),
		ConsumerGroupID: conf.Get("CONSUMER_ID"),
		OffSet:          c.getIntConfig(conf, "PUBSUB_OFFSET", -1),
		BatchSize:       c.getIntConfig(conf, "KAFKA_BATCH_SIZE", kafka.DefaultBatchSize),
		BatchBytes:      c.getIntConfig(conf, "KAFKA_BATCH_BYTES", kafka.DefaultBatchBytes),
		BatchTimeout:    c.getIntConfig(conf, "KAFKA_BATCH_TIMEOUT", kafka.DefaultBatchTimeout),
	}
}

func (c *Container) initializeGoogle(conf config.Config) {
	c.PubSub = google.New(google.Config{
		ProjectID:        conf.Get("GOOGLE_PROJECT_ID"),
		SubscriptionName: conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
	}, c.Logger, c.metricsManager)
}

func (c *Container) initializeNATS(conf config.Config) {
	natsConfig := &nats.Config{
		Server: conf.Get("PUBSUB_BROKER"),
		Stream: nats.StreamConfig{
			Stream:   conf.Get("NATS_STREAM"),
			Subjects: strings.Split(conf.Get("NATS_SUBJECTS"), ","),
		},
		MaxWait:     c.getDuration(conf, "NATS_MAX_WAIT"),
		BatchSize:   c.getIntConfig(conf, "NATS_BATCH_SIZE", 0),
		MaxPullWait: c.getIntConfig(conf, "NATS_MAX_PULL_WAIT", 0),
		Consumer:    conf.Get("NATS_CONSUMER"),
	}

	var err error

	c.PubSub, err = nats.New(natsConfig, c.Logger, c.metricsManager)
	if err != nil {
		c.Logger.Error("failed to create NATS client: %v", err)
	}
}

func (c *Container) initializeFile() {
	c.File = file.New(c.Logger)
}

func (c *Container) getIntConfig(conf config.Config, key string, defaultValue int) int {
	value, err := strconv.Atoi(conf.GetOrDefault(key, strconv.Itoa(defaultValue)))
	if err != nil {
		c.Logger.Errorf("invalid value for %s: %v", key, err)

		return defaultValue
	}

	return value
}

func (c *Container) getDuration(conf config.Config, key string) time.Duration {
	value, err := time.ParseDuration(conf.Get(key))
	if err != nil {
		c.Logger.Errorf("invalid value for %s: %v", key, err)

		return 0
	}

	return value
}

func (c *Container) Close() error {
	var err error

	if !isNil(c.SQL) {
		err = errors.Join(err, c.SQL.Close())
	}

	if !isNil(c.Redis) {
		err = errors.Join(err, c.Redis.Close())
	}

	if !isNil(c.PubSub) {
		err = errors.Join(err, c.PubSub.Close())
	}

	return err
}

func (c *Container) createMqttPubSub(conf config.Config) pubsub.Client {
	var qos byte

	port, _ := strconv.Atoi(conf.Get("MQTT_PORT"))
	order, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_MESSAGE_ORDER", "false"))

	keepAlive, err := time.ParseDuration(conf.Get("MQTT_KEEP_ALIVE"))
	if err != nil {
		keepAlive = 30 * time.Second

		c.Logger.Debug("MQTT_KEEP_ALIVE is not set or ivalid, setting it to 30 seconds")
	}

	switch conf.Get("MQTT_QOS") {
	case "1":
		qos = 1
	case "2":
		qos = 2
	default:
		qos = 0
	}

	configs := &mqtt.Config{
		Protocol:     conf.GetOrDefault("MQTT_PROTOCOL", "tcp"), // using tcp as default method to connect to broker
		Hostname:     conf.Get("MQTT_HOST"),
		Port:         port,
		Username:     conf.Get("MQTT_USER"),
		Password:     conf.Get("MQTT_PASSWORD"),
		ClientID:     conf.Get("MQTT_CLIENT_ID_SUFFIX"),
		QoS:          qos,
		Order:        order,
		KeepAlive:    keepAlive,
		CloseTimeout: 0 * time.Millisecond,
	}

	return mqtt.New(configs, c.Logger, c.metricsManager)
}

// GetHTTPService returns registered HTTP services.
// HTTP services are registered from AddHTTPService method of GoFr object.
func (c *Container) GetHTTPService(serviceName string) service.HTTP {
	return c.Services[serviceName]
}

func (c *Container) Metrics() metrics.Manager {
	return c.metricsManager
}

func (c *Container) registerFrameworkMetrics() {
	// system info metrics
	c.Metrics().NewGauge("app_info", "Info for app_name, app_version and framework_version.")
	c.Metrics().NewGauge("app_go_routines", "Number of Go routines running.")
	c.Metrics().NewGauge("app_sys_memory_alloc", "Number of bytes allocated for heap objects.")
	c.Metrics().NewGauge("app_sys_total_alloc", "Number of cumulative bytes allocated for heap objects.")
	c.Metrics().NewGauge("app_go_numGC", "Number of completed Garbage Collector cycles.")
	c.Metrics().NewGauge("app_go_sys", "Number of total bytes of memory.")

	{ // HTTP metrics
		httpBuckets := []float64{.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30}
		c.Metrics().NewHistogram("app_http_response", "Response time of HTTP requests in seconds.", httpBuckets...)
		c.Metrics().NewHistogram("app_http_service_response", "Response time of HTTP service requests in seconds.", httpBuckets...)
	}

	{ // Redis metrics
		redisBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 1.25, 1.5, 2, 2.5, 3}
		c.Metrics().NewHistogram("app_redis_stats", "Response time of Redis commands in milliseconds.", redisBuckets...)
	}

	{ // SQL metrics
		sqlBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
		c.Metrics().NewHistogram("app_sql_stats", "Response time of SQL queries in milliseconds.", sqlBuckets...)
		c.Metrics().NewGauge("app_sql_open_connections", "Number of open SQL connections.")
		c.Metrics().NewGauge("app_sql_inUse_connections", "Number of inUse SQL connections.")
	}

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

func (*Container) GetConnectionFromContext(ctx context.Context) *websocket.Connection {
	conn, ok := ctx.Value(websocket.WSConnectionKey).(*websocket.Connection)
	if !ok {
		return nil
	}

	return conn
}
