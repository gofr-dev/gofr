package gofr

import (
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
)

const (
	maxIdleReplicaCapDefault  = 50
	minIdleReplicaDefault     = 10
	maxOpenReplicaCapDefault  = 200
	minOpenReplicaDefault     = 50
	defaultIdleReplicaDefault = 10
	defaultOpenReplicaDefault = 100
)

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(db container.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-mongo")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Mongo = db
}

// AddFTP sets the FTP datasource in the app's container.
// Deprecated: Use the AddFile method instead.
func (a *App) AddFTP(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddPubSub sets the PubSub client in the app's container.
func (a *App) AddPubSub(pubsub container.PubSubProvider) {
	pubsub.UseLogger(a.Logger())
	pubsub.UseMetrics(a.Metrics())

	pubsub.Connect()

	a.container.PubSub = pubsub
}

// AddFileStore sets the FTP,SFTP,S3 datasource in the app's container.
func (a *App) AddFileStore(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package : gofr.dev/pkg/gofr/datasource/clickhouse .
func (a *App) AddClickhouse(db container.ClickhouseProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-clickhouse")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Clickhouse = db
}

// AddOracle initializes the OracleDB client.
// Official implementation is available in the package: gofr.dev/pkg/gofr/datasource/oracle.
func (a *App) AddOracle(db container.OracleProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-oracle")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Oracle = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the AddMongo method instead.
func (a *App) UseMongo(db container.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(db container.CassandraProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-cassandra")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Cassandra = db
}

// AddKVStore sets the KV-Store datasource in the app's container.
func (a *App) AddKVStore(db container.KVStoreProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-kvstore")

	db.UseTracer(tracer)

	db.Connect()

	a.container.KVStore = db
}

// AddSolr sets the Solr datasource in the app's container.
func (a *App) AddSolr(db container.SolrProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-solr")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Solr = db
}

// AddDgraph sets the Dgraph datasource in the app's container.
func (a *App) AddDgraph(db container.DgraphProvider) {
	// Create the Dgraph client with the provided configuration
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-dgraph")

	db.UseTracer(tracer)

	db.Connect()

	a.container.DGraph = db
}

// AddOpenTSDB sets the OpenTSDB datasource in the app's container.
func (a *App) AddOpenTSDB(db container.OpenTSDBProvider) {
	// Create the Opentsdb client with the provided configuration
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-opentsdb")

	db.UseTracer(tracer)

	db.Connect()

	a.container.OpenTSDB = db
}

// AddScyllaDB sets the ScyllaDB datasource in the app's container.
func (a *App) AddScyllaDB(db container.ScyllaDBProvider) {
	// Create the ScyllaDB client with the provided configuration
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-scylladb")
	db.UseTracer(tracer)
	db.Connect()
	a.container.ScyllaDB = db
}

// AddArangoDB sets the ArangoDB datasource in the app's container.
func (a *App) AddArangoDB(db container.ArangoDBProvider) {
	// Set up logger, metrics, and tracer
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	// Get tracer from OpenTelemetry
	tracer := otel.GetTracerProvider().Tracer("gofr-arangodb")
	db.UseTracer(tracer)

	// Connect to ArangoDB
	db.Connect()

	// Add the ArangoDB provider to the container
	a.container.ArangoDB = db
}

func (a *App) AddSurrealDB(db container.SurrealBDProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-surrealdb")
	db.UseTracer(tracer)
	db.Connect()
	a.container.SurrealDB = db
}

func (a *App) AddElasticsearch(db container.ElasticsearchProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-elasticsearch")
	db.UseTracer(tracer)
	db.Connect()

	a.container.Elasticsearch = db
}

func (a *App) AddCouchbase(db container.CouchbaseProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-couchbase")
	db.UseTracer(tracer)
	db.Connect()

	a.container.Couchbase = db
}

// AddDBResolver sets up read/write splitting for SQL databases.
func (a *App) AddDBResolver(resolver container.DBResolverProvider) {
	// Exit if SQL is not configured.
	if a.container.SQL == nil {
		a.Logger().Errorf("Cannot set up DB resolver: SQL is not configured")
		return
	}

	// Set up logger, metrics, tracer.
	resolver.UseLogger(a.Logger())
	resolver.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr.dbresolver")
	resolver.UseTracer(tracer)

	// Connect (no-op for resolver)
	resolver.Connect()

	// Create replica connections.
	replicas := createReplicaConnections(a.Config, a.Logger(), a.Metrics())
	if len(replicas) == 0 {
		a.Logger().Debugf("No replicas configured, skipping DB resolver setup")
		return
	}

	// Build resolver with primary and replicas.
	resolverDB, err := resolver.Build(a.container.SQL, replicas)
	if err != nil {
		a.Logger().Errorf("Failed to build DB resolver: %v", err)
		return
	}

	a.container.SQL = resolverDB

	a.UseMiddleware()

	a.Logger().Logf("DB read/write splitting enabled with %d replicas", len(replicas))
}

// createReplicaConnections creates optimized DB connections to replicas.
func createReplicaConnections(cfg config.Config, logger logging.Logger, mtrcs metrics.Manager) []container.DB {
	hosts := strings.Split(cfg.Get("DB_REPLICA_HOSTS"), ",")
	ports := strings.Split(cfg.Get("DB_REPLICA_PORTS"), ",")
	users := strings.Split(cfg.Get("DB_REPLICA_USERS"), ",")
	passwords := strings.Split(cfg.Get("DB_REPLICA_PASSWORDS"), ",")

	// Ensure minimum: at least hosts must be defined
	if len(hosts) == 0 || (len(hosts) == 1 && hosts[0] == "") {
		return nil
	}

	var replicas []container.DB

	for i, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		port := safeGet(ports, i, cfg.Get("DB_PORT"))
		user := safeGet(users, i, cfg.Get("DB_USER"))
		pass := safeGet(passwords, i, cfg.Get("DB_PASSWORD"))

		// Wrap replica config
		replicaConfig := &replicaConfigWrapper{
			Config:   cfg,
			host:     host,
			port:     port,
			user:     user,
			password: pass,
		}

		replica := sql.NewSQL(replicaConfig, logger, mtrcs)
		if replica != nil {
			replicas = append(replicas, replica)

			logger.Logf("Created DB replica connection to %s:%s as user %s", host, port, user)
		}
	}

	return replicas
}

// safeGet returns the element at index i if present, otherwise fallback.
func safeGet(list []string, i int, fallback string) string {
	if i < len(list) && strings.TrimSpace(list[i]) != "" {
		return strings.TrimSpace(list[i])
	}

	return fallback
}

// replicaConfigWrapper wraps config and optimizes connection settings for replicas.
type replicaConfigWrapper struct {
	config.Config
	host     string
	port     string
	user     string
	password string
}

// Get overrides config values for replica optimization.
func (c *replicaConfigWrapper) Get(key string) string {
	switch key {
	case "DB_HOST":
		return c.host
	case "DB_PORT":
		return c.port
	case "DB_USER":
		return c.user
	case "DB_PASSWORD":
		return c.password
	case "DB_MAX_IDLE_CONNECTION":
		return optimizedIdleConnections(c.Config)
	case "DB_MAX_OPEN_CONNECTION":
		return optimizedOpenConnections(c.Config)
	default:
		return c.Config.Get(key)
	}
}

func getReplicaConfigInt(cfg config.Config, key string, fallback int) int {
	valStr := cfg.Get(key)

	if valStr == "" {
		return fallback
	}

	val, err := strconv.Atoi(valStr)

	if err != nil || val <= 0 {
		return fallback
	}

	return val
}

func optimizedIdleConnections(cfg config.Config) string {
	// Load caps from config or fallback to defaults
	maxCap := getReplicaConfigInt(cfg, "DB_REPLICA_MAX_IDLE_CONNECTIONS", maxIdleReplicaCapDefault)
	minVal := getReplicaConfigInt(cfg, "DB_REPLICA_MIN_IDLE_CONNECTIONS", minIdleReplicaDefault)
	defaultVal := getReplicaConfigInt(cfg, "DB_REPLICA_DEFAULT_IDLE_CONNECTIONS", defaultIdleReplicaDefault)

	val, err := strconv.Atoi(cfg.Get("DB_MAX_IDLE_CONNECTION"))

	if err != nil || val <= 0 {
		return strconv.Itoa(defaultVal)
	}

	optimized := val * 4

	switch {
	case optimized > maxCap:
		optimized = maxCap
	case optimized < minVal:
		optimized = minVal
	}

	return strconv.Itoa(optimized)
}

func optimizedOpenConnections(cfg config.Config) string {
	// Load caps from config or fallback to defaults
	maxCap := getReplicaConfigInt(cfg, "DB_REPLICA_MAX_OPEN_CONNECTIONS", maxOpenReplicaCapDefault)
	minVal := getReplicaConfigInt(cfg, "DB_REPLICA_MIN_OPEN_CONNECTIONS", minOpenReplicaDefault)
	defaultVal := getReplicaConfigInt(cfg, "DB_REPLICA_DEFAULT_OPEN_CONNECTIONS", defaultOpenReplicaDefault)

	val, err := strconv.Atoi(cfg.Get("DB_MAX_OPEN_CONNECTION"))

	if err != nil {
		return strconv.Itoa(defaultVal)
	}

	if val == 0 {
		return strconv.Itoa(defaultVal)
	}

	optimized := val * 2

	switch {
	case optimized > maxCap:
		optimized = maxCap
	case optimized < minVal:
		optimized = minVal
	}

	return strconv.Itoa(optimized)
}

func (a *App) AddInfluxDB(db container.InfluxDBProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-influxdb")
	db.UseTracer(tracer)
	db.Connect()

	a.container.InfluxDB = db
}
