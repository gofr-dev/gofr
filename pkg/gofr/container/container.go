/*
Package container provides a centralized structure to manage common application-level concerns such as
logging, connection pools, and service management. This package is designed to facilitate the sharing and
management of these concerns across different parts of an application.

Supported data sources:
  - Databases (Cassandra, ClickHouse, MongoDB, DGraph, MySQL, PostgreSQL, SQLite)
  - Key-value storages (Redis, BadgerDB)
  - Pub/Sub systems (Azure Event Hub, Google as backend, Kafka, MQTT)
  - Search engines (Solr)
  - File systems (FTP, SFTP, S3, GCS, Azure File Storage)
*/
package container

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // This is required to be blank import
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
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
	"gofr.dev/pkg/gofr/websocket"
)

const (
	redisPubSubModeStreams = "streams"
	redisPubSubModePubSub  = "pubsub"
)

// Container is a collection of all common application level concerns. Things like Logger, Connection Pool for Redis
// etc. which is shared across is placed here.
type Container struct {
	logging.Logger

	appName    string
	appVersion string

	Services        map[string]service.HTTP
	metricsManager  metrics.Manager
	shutdownMetrics exporters.ShutdownFunc
	PubSub          pubsub.Client

	WSManager *websocket.Manager

	Redis Redis
	SQL   DB

	Cassandra     CassandraWithContext
	Clickhouse    Clickhouse
	Mongo         Mongo
	Solr          Solr
	DGraph        Dgraph
	OpenTSDB      OpenTSDB
	ScyllaDB      ScyllaDB
	SurrealDB     SurrealDB
	ArangoDB      ArangoDB
	Elasticsearch Elasticsearch
	Oracle        OracleDB
	Couchbase     Couchbase
	InfluxDB      InfluxDB

	KVStore KVStore

	File file.FileSystem

	llms      map[string]ai.LLM   // instrumented models by name ("" = default); a mock sets the default directly
	llmModels map[string]ai.Model // raw providers by name, for uninstrumented health probes
	tools     ai.Tools            // set by app.EnableMCP so ctx.LLM().Tools() exposes the service's own handlers
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
	if c.appName == "" {
		c.appName = conf.GetOrDefault("APP_NAME", "gofr-app")
	}

	if c.appVersion == "" {
		c.appVersion = conf.GetOrDefault("APP_VERSION", "dev")
	}

	if c.Logger == nil {
		levelFetchConfig, err := strconv.Atoi(conf.GetOrDefault("REMOTE_LOG_FETCH_INTERVAL", "15"))
		if err != nil {
			levelFetchConfig = 15
		}

		c.Logger = remotelogger.New(logging.GetLevelFromString(conf.Get("LOG_LEVEL")), conf.Get("REMOTE_LOG_URL"),
			time.Duration(levelFetchConfig)*time.Second)

		if err != nil {
			c.Logger.Error("invalid value for REMOTE_LOG_FETCH_INTERVAL. setting default of 15 sec.")
		}
	}

	c.Logger.Debug("Container is being created")

	metricsCfg := metricsExporterConfig(conf, c.GetAppName(), c.GetAppVersion(), c.Logger)
	shutdown, meter := exporters.Build(context.Background(), &metricsCfg, c.Logger)
	c.shutdownMetrics = shutdown
	c.metricsManager = metrics.NewMetricsManager(meter, c.Logger)

	exporters.SendFrameworkStartupTelemetry(c.GetAppName(), c.GetAppVersion())

	// Register framework metrics
	c.registerFrameworkMetrics()

	// Populating an instance of app_info with the app details, the value is set as 1 to depict the no. of instances
	c.Metrics().SetGauge("app_info", 1,
		"app_name", c.GetAppName(), "app_version", c.GetAppVersion(), "framework_version", version.Framework)

	c.Redis = redis.NewClient(conf, c.Logger, c.metricsManager)

	c.SQL = sql.NewSQL(conf, c.Logger, c.metricsManager)

	c.createPubSub(conf)

	c.File = file.NewLocalFileSystem(c.Logger)

	c.WSManager = websocket.New()
}

func (c *Container) createPubSub(conf config.Config) {
	switch strings.ToUpper(conf.Get("PUBSUB_BACKEND")) {
	case "KAFKA":
		c.createKafkaPubSub(conf)
	case "GOOGLE":
		c.createGooglePubSub(conf)
	case "MQTT":
		c.PubSub = c.createMqttPubSub(conf)
	case "REDIS":
		c.createRedisPubSub(conf)
	}
}

// ShutdownMetrics flushes pending metrics and shuts down the metrics provider.
// It is safe to call when no provider was configured, and idempotent (see
// exporters.Build for the flush/idempotency guarantees). Push exporters (e.g.
// OTLP) rely on this to emit their final window before the process exits.
func (c *Container) ShutdownMetrics(ctx context.Context) error {
	if c.shutdownMetrics == nil {
		return nil
	}

	return c.shutdownMetrics(ctx)
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

	for _, conn := range c.WSManager.ListConnections() {
		c.WSManager.CloseConnection(conn)
	}

	return err
}

func (c *Container) createMqttPubSub(conf config.Config) pubsub.Client {
	var qos byte

	port, err := strconv.Atoi(conf.GetOrDefault("MQTT_PORT", "0"))
	if err != nil {
		c.Logger.Error("Invalid value for MQTT_PORT, using default: 0")
	}

	order, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_MESSAGE_ORDER", "false"))

	retrieveRetained, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_RETRIEVE_RETAINED", "false"))

	keepAlive, err := time.ParseDuration(conf.Get("MQTT_KEEP_ALIVE"))
	if err != nil {
		keepAlive = 30 * time.Second

		c.Logger.Debug("MQTT_KEEP_ALIVE is not set or invalid, setting it to 30 seconds")
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
		Protocol:         conf.GetOrDefault("MQTT_PROTOCOL", "tcp"), // using tcp as default method to connect to broker
		Hostname:         conf.Get("MQTT_HOST"),
		Port:             port,
		Username:         conf.Get("MQTT_USER"),
		Password:         conf.Get("MQTT_PASSWORD"),
		ClientID:         conf.Get("MQTT_CLIENT_ID_SUFFIX"),
		QoS:              qos,
		Order:            order,
		RetrieveRetained: retrieveRetained,
		KeepAlive:        keepAlive,
		CloseTimeout:     0 * time.Millisecond,
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

// SetLLM stores the model added via app.AddLLM, building the instrumented ai.LLM once. The tracer
// is available here because initTracer runs before any AddLLM; the tool provider is resolved lazily
// so tools registered later (by EnableMCP) are still visible via ctx.LLM().Tools().
func (c *Container) SetLLM(m ai.Model, name ...string) {
	if c.llmModels == nil {
		c.llmModels = map[string]ai.Model{}
		c.llms = map[string]ai.LLM{}
	}

	n := llmName(name)
	c.llmModels[n] = m
	c.llms[n] = ai.NewLLM(m, ai.Deps{
		Metrics: c.metricsManager,
		Tracer:  otel.GetTracerProvider().Tracer("gofr-llm"),
		Logger:  c.Logger,
		Tools:   lazyTools{c},
	})
}

// LLMModel returns the raw model added via app.AddLLM (the default, or the one named by WithName),
// or nil if none was added. Prefer LLM() in handlers; this exposes the uninstrumented provider for
// advanced use.
func (c *Container) LLMModel(name ...string) ai.Model {
	return c.llmModels[llmName(name)]
}

// HasLLM reports whether any model has been registered (under any name). Used to register the LLM
// metrics exactly once, on the first model added.
func (c *Container) HasLLM() bool {
	return len(c.llms) > 0
}

// llmName resolves the optional variadic model name to the default ("") when omitted.
func llmName(name []string) string {
	if len(name) > 0 {
		return name[0]
	}

	return ""
}

// SetTools installs the agent-tool provider exposed to handlers via ctx.LLM().Tools().
func (c *Container) SetTools(t ai.Tools) {
	c.tools = t
}

// LLM returns the instrumented model exposed to handlers. For an unknown or absent model name it
// returns a not-configured LLM whose calls fail with ai.ErrLLMNotConfigured, rather than a nil
// interface, so a typo'd or missing name gives a clear error instead of a nil-pointer panic. A mock
// container injects its mock through the same field.
func (c *Container) LLM(name ...string) ai.LLM {
	if llm := c.llms[llmName(name)]; llm != nil {
		return llm
	}

	return notConfiguredLLM{c: c}
}

// notConfiguredLLM is the nil-safe LLM returned by LLM(name) for an unknown or absent model name. The
// model calls fail with ai.ErrLLMNotConfigured; Tools() still resolves the service's own tools, which
// are independent of any registered model.
type notConfiguredLLM struct{ c *Container }

func (notConfiguredLLM) Chat(context.Context, []ai.Message, ...ai.Option) (*ai.Response, error) {
	return nil, ai.ErrLLMNotConfigured
}

func (notConfiguredLLM) Generate(context.Context, string, ...ai.Option) (*ai.Response, error) {
	return nil, ai.ErrLLMNotConfigured
}

func (notConfiguredLLM) Stream(context.Context, []ai.Message, ...ai.Option) (ai.Streamer, error) {
	return nil, ai.ErrLLMNotConfigured
}

func (notConfiguredLLM) HealthCheck(context.Context) datasource.Health {
	return datasource.Health{Status: datasource.StatusDown}
}

func (notConfiguredLLM) Name() string { return "" }

// Tools resolves the service's own tools, which exist independently of any model. notConfiguredLLM
// and lazyTools both wrap only *Container, so the conversion reuses that late-binding plumbing.
func (n notConfiguredLLM) Tools() ai.Tools { return lazyTools(n) }

// lazyTools resolves the container's tool provider at call time, so tools registered after the LLM
// (for example by EnableMCP) are still visible via ctx.LLM().Tools().
type lazyTools struct{ c *Container }

func (l lazyTools) List() []ai.ToolSpec {
	if l.c.tools == nil {
		return nil
	}

	return l.c.tools.List()
}

func (l lazyTools) Only(names ...string) ai.Tools {
	if l.c.tools == nil {
		return l
	}

	return l.c.tools.Only(names...)
}

func (l lazyTools) Call(ctx context.Context, name string, args json.RawMessage) (ai.Result, error) {
	if l.c.tools == nil {
		return ai.Result{}, ai.ErrToolNotFound
	}

	return l.c.tools.Call(ctx, name, args)
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
		c.Metrics().NewCounter("app_http_retry_count", "Total number of retry events")
		c.Metrics().NewGauge("app_http_circuit_breaker_state", "Current state of the circuit breaker (0 for Closed, 1 for Open)")
	}

	{ // Redis metrics
		// Redis records latency in microseconds (see datasource/redis hook), so it uses the microsecond buckets.
		redisBuckets := getDefaultDatasourceMicrosecondBuckets()
		c.Metrics().NewHistogram("app_redis_stats", "Response time of Redis commands in microseconds.", redisBuckets...)
	}

	{ // SQL metrics
		sqlBuckets := getDefaultDatasourceBuckets()
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

// GetConnectionFromContext retrieves a WebSocket connection from the context using the Manager.
func (c *Container) GetConnectionFromContext(ctx context.Context) *websocket.Connection {
	if c.WSManager == nil {
		return nil
	}

	// First check if connection is directly stored in context
	if conn, ok := ctx.Value(websocket.WSConnectionKey).(*websocket.Connection); ok {
		return conn
	}

	// Fallback to connection ID lookup
	if connID, ok := ctx.Value(websocket.WSConnectionKey).(string); ok {
		return c.WSManager.GetWebsocketConnection(connID)
	}

	return nil
}

// GetWSConnectionByServiceName retrieves a WebSocket connection by its service name.
func (c *Container) GetWSConnectionByServiceName(serviceName string) *websocket.Connection {
	return c.WSManager.GetConnectionByServiceName(serviceName)
}

// AddConnection adds a WebSocket connection to the Manager.
func (c *Container) AddConnection(connID string, conn *websocket.Connection) {
	c.WSManager.AddWebsocketConnection(connID, conn)
}

// RemoveConnection removes a WebSocket connection from the Manager.
func (c *Container) RemoveConnection(connID string) {
	c.WSManager.CloseConnection(connID)
}

// getDefaultDatasourceBuckets returns the standard histogram buckets, in milliseconds, for datasources
// that record latency in milliseconds (e.g. SQL). Covers 50µs-3min to span everything from fast
// in-memory hits to long-running queries near request-timeout boundaries, giving consistent
// observability across SQL and other millisecond-based datasources.
func getDefaultDatasourceBuckets() []float64 {
	return []float64{
		.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 5, 7.5, 10, // 50µs-10ms
		25, 50, 100, 250, 500, 1000, 5000, 10000, 30000, 60000, 120000, 180000, // 25ms-3min
	}
}

// getDefaultDatasourceMicrosecondBuckets mirrors getDefaultDatasourceBuckets scaled to microseconds,
// for datasources that record latency in microseconds (e.g. Redis). Covers 50µs-3min.
func getDefaultDatasourceMicrosecondBuckets() []float64 {
	return []float64{
		50, 75, 100, 125, 150, 200, 300, 500, 750, 1000, 2000, 3000, 5000, 7500, 10000, // 50µs-10ms
		25000, 50000, 100000, 250000, 500000, 1000000, 5000000, 10000000, 30000000, 60000000, 120000000, 180000000, // 25ms-3min
	}
}

func (c *Container) createKafkaPubSub(conf config.Config) {
	if conf.Get("PUBSUB_BROKER") == "" {
		return
	}

	partition, err := strconv.Atoi(conf.GetOrDefault("PARTITION_SIZE", "0"))
	if err != nil {
		c.Logger.Error("Invalid value for PARTITION_SIZE, using default: 0")
	}

	// PUBSUB_OFFSET determines the starting position for message consumption in Kafka.
	// This allows control over whether to read historical messages or only new ones:
	// - Default value -1: Start from the latest offset (only consume new messages after consumer starts)
	// - Value 0: Start from the earliest offset (read all historical messages from the beginning)
	// - Positive value: Start from a specific offset position (useful for resuming from a known point)
	// This is particularly important for scenarios like message replay, recovery from failures,
	// or when you only want to process messages that arrive after the consumer is initialized.
	offSet, err := strconv.Atoi(conf.GetOrDefault("PUBSUB_OFFSET", "-1"))
	if err != nil {
		c.Logger.Error("Invalid value for PUBSUB_OFFSET, using default: -1")
	}

	batchSize, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_SIZE", strconv.Itoa(kafka.DefaultBatchSize)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_SIZE, using default: %d", kafka.DefaultBatchSize)
	}

	batchBytes, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_BYTES", strconv.Itoa(kafka.DefaultBatchBytes)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_BYTES, using default: %d", kafka.DefaultBatchBytes)
	}

	batchTimeout, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_TIMEOUT", strconv.Itoa(kafka.DefaultBatchTimeout)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_TIMEOUT, using default: %d", kafka.DefaultBatchTimeout)
	}

	tlsConf := kafka.TLSConfig{
		CertFile:           conf.Get("KAFKA_TLS_CERT_FILE"),
		KeyFile:            conf.Get("KAFKA_TLS_KEY_FILE"),
		CACertFile:         conf.Get("KAFKA_TLS_CA_CERT_FILE"),
		InsecureSkipVerify: conf.Get("KAFKA_TLS_INSECURE_SKIP_VERIFY") == "true",
	}

	pubsubBrokers := strings.Split(conf.Get("PUBSUB_BROKER"), ",")

	c.PubSub = kafka.New(&kafka.Config{
		Brokers:          pubsubBrokers,
		Partition:        partition,
		ConsumerGroupID:  conf.Get("CONSUMER_ID"),
		OffSet:           offSet,
		BatchSize:        batchSize,
		BatchBytes:       batchBytes,
		BatchTimeout:     batchTimeout,
		SecurityProtocol: conf.Get("KAFKA_SECURITY_PROTOCOL"),
		SASLMechanism:    conf.Get("KAFKA_SASL_MECHANISM"),
		SASLUser:         conf.Get("KAFKA_SASL_USERNAME"),
		SASLPassword:     conf.Get("KAFKA_SASL_PASSWORD"),
		TLS:              tlsConf,
	}, c.Logger, c.metricsManager)
}

func (c *Container) createGooglePubSub(conf config.Config) {
	c.PubSub = google.New(google.Config{
		ProjectID:        conf.Get("GOOGLE_PROJECT_ID"),
		SubscriptionName: conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
	}, c.Logger, c.metricsManager)
}

func (c *Container) createRedisPubSub(conf config.Config) {
	c.warnIfRedisPubSubSharesRedisDB(conf)

	// Redis PubSub is initialized via NewPubSub constructor, aligning with other PubSub implementations.
	c.PubSub = redis.NewPubSub(conf, c.Logger, c.metricsManager)
}

func (c *Container) warnIfRedisPubSubSharesRedisDB(conf config.Config) {
	// Warn (do not fail): if Redis PubSub (streams mode) shares the same Redis logical DB as the primary Redis datasource,
	// GoFr migrations can later fail due to `gofr_migrations` key-type collision (HASH vs STREAM).
	if isNil(c.Redis) || effectiveRedisPubSubMode(conf) != redisPubSubModeStreams {
		return
	}

	redisDBStr := conf.Get("REDIS_DB")
	if redisDBStr == "" {
		redisDBStr = "0"
	}

	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		redisDB = 0
	}

	pubsubDBStr := conf.Get("REDIS_PUBSUB_DB")
	if pubsubDBStr == "" {
		// No warning needed - defaults to DB 15 which is safe and different from typical REDIS_DB (0)
		return
	}

	// Only warn if user explicitly set it to the same as REDIS_DB
	pubsubDB, err := strconv.Atoi(pubsubDBStr)
	if err != nil {
		// Invalid value - will use default DB 15, no warning needed
		return
	}

	if pubsubDB == redisDB {
		c.Logger.Warnf(
			"REDIS_PUBSUB_DB (%d) is the same as REDIS_DB (%d); migrations may fail (gofr_migrations HASH/STREAM). "+
				"Set REDIS_PUBSUB_DB to a different DB.",
			pubsubDB, redisDB,
		)
	}
}

func effectiveRedisPubSubMode(conf config.Config) string {
	mode := strings.ToLower(conf.Get("REDIS_PUBSUB_MODE"))
	if mode == redisPubSubModePubSub {
		return redisPubSubModePubSub
	}

	// Default and fallback is streams.
	return redisPubSubModeStreams
}
