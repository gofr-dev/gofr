package gofr

import (
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"strconv"
	"strings"
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

// AddDBResolver sets up read/write splitting for SQL databases
func (a *App) AddDBResolver(resolver container.DBResolverProvider) {
	// Exit if SQL is not configured
	if a.container.SQL == nil {
		a.Logger().Errorf("Cannot set up DB resolver: SQL is not configured")
		return
	}

	// Set up logger, metrics, tracer
	resolver.UseLogger(a.Logger())
	resolver.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr.dbresolver")
	resolver.UseTracer(tracer)

	// Connect (no-op for resolver)
	resolver.Connect()

	// Create replica connections
	replicas := createReplicaConnections(a.Config, a.Logger(), a.Metrics())
	if len(replicas) == 0 {
		a.Logger().Debugf("No replicas configured, skipping DB resolver setup")
		return
	}

	// Build resolver with primary and replicas
	resolverDB, err := resolver.Build(a.container.SQL, replicas)
	if err != nil {
		a.Logger().Errorf("Failed to build DB resolver: %v", err)
		return
	}

	a.container.SQL = resolverDB
	a.Logger().Logf("DB read/write splitting enabled with %d replicas", len(replicas))
}

// createReplicaConnections creates optimized DB connections to replicas
func createReplicaConnections(config config.Config, logger logging.Logger, metrics metrics.Manager) []container.DB {
	replicaHostsStr := config.Get("DB_REPLICA_HOSTS")
	if replicaHostsStr == "" {
		return nil
	}

	replicaHosts := strings.Split(replicaHostsStr, ",")
	var replicas []container.DB

	for _, host := range replicaHosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		// Create optimized replica config
		replicaConfig := &replicaConfigWrapper{
			Config:     config,
			hostString: host,
		}

		replica := sql.NewSQL(replicaConfig, logger, metrics)
		if replica != nil {
			replicas = append(replicas, replica)
			logger.Logf("Created DB replica connection to %s", host)
		}
	}

	return replicas
}

// replicaConfigWrapper wraps config and optimizes connection settings for replicas
type replicaConfigWrapper struct {
	config.Config
	hostString string
}

// Get overrides config values for replica optimization
func (c *replicaConfigWrapper) Get(key string) string {
	switch key {
	case "DB_HOST":
		if strings.Contains(c.hostString, ":") {
			return strings.Split(c.hostString, ":")[0]
		}
		return c.hostString
	case "DB_PORT":
		if strings.Contains(c.hostString, ":") {
			parts := strings.Split(c.hostString, ":")
			if len(parts) > 1 {
				return parts[1]
			}
		}
		if replicaPort := c.Config.Get("DB_REPLICA_PORT"); replicaPort != "" {
			return replicaPort
		}
		return c.Config.Get("DB_PORT")
	case "DB_USER":
		if replicaUser := c.Config.Get("DB_REPLICA_USER"); replicaUser != "" {
			return replicaUser
		}
	case "DB_PASSWORD":
		if replicaPass := c.Config.Get("DB_REPLICA_PASSWORD"); replicaPass != "" {
			return replicaPass
		}
	case "DB_MAX_IDLE_CONNECTION":
		// Simple optimization: 4x idle connections for read replicas
		if primaryIdle := c.Config.Get("DB_MAX_IDLE_CONNECTION"); primaryIdle != "" {
			if val, err := strconv.Atoi(primaryIdle); err == nil && val > 0 {
				optimized := val * 4
				if optimized > 50 { // Cap at 50
					optimized = 50
				}
				if optimized < 10 { // Min of 10 for read performance
					optimized = 10
				}
				return strconv.Itoa(optimized)
			}
		}
		return "10" // Default optimized value
	case "DB_MAX_OPEN_CONNECTION":
		// Simple optimization: 2x max connections for read replicas
		if primaryOpen := c.Config.Get("DB_MAX_OPEN_CONNECTION"); primaryOpen != "" {
			if val, err := strconv.Atoi(primaryOpen); err == nil {
				if val == 0 { // Handle unlimited
					return "100" // Reasonable limit for replicas
				}
				optimized := val * 2
				if optimized > 200 { // Cap at 200
					optimized = 200
				}
				if optimized < 50 { // Min of 50 for read distribution
					optimized = 50
				}
				return strconv.Itoa(optimized)
			}
		}
		return "100" // Default optimized value
	}

	return c.Config.Get(key)
}
