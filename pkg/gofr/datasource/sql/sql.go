package sql

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq" // used for concrete implementation of the database driver.
	_ "modernc.org/sqlite"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	sqlite            = "sqlite"
	cockroachDB       = "cockroachdb"
	defaultDBPort     = 3306
	requireSSLMode    = "require"
	tlsSkipVerify     = "tls=skip-verify"
	sslModeDisable    = "disable"
	sslModeVerifyCA   = "verify-ca"
	sslModeVerifyFull = "verify-full"
	tlsCustom         = "tls=custom"
	localhost         = "localhost"
)

var (
	errUnsupportedDialect = fmt.Errorf(
		"unsupported db dialect; supported dialects are - mysql, postgres, supabase, sqlite, %s", cockroachDB)
	errFailedCACerts = fmt.Errorf("failed to append CA certificate")
)

// DBConfig has those members which are necessary variables while connecting to database.
type DBConfig struct {
	Dialect     string
	HostName    string
	User        string
	Password    string
	Port        string
	Database    string
	SSLMode     string
	MaxIdleConn int
	MaxOpenConn int
	Charset     string
}

// redactedPassword is the fixed mask substituted for a non-empty password whenever
// a DBConfig is stringified, so the raw secret is never printed.
const redactedPassword = "*****"

// String implements fmt.Stringer so that logging a DBConfig — directly or as part
// of a larger struct, with %v or %+v — never leaks the raw password. The Password
// field is redacted to a fixed mask while every other field is preserved, keeping
// the output useful for diagnosing connection issues.
func (c DBConfig) String() string {
	password := ""
	if c.Password != "" {
		password = redactedPassword
	}

	return fmt.Sprintf("{Dialect:%s HostName:%s User:%s Password:%s Port:%s Database:%s "+
		"SSLMode:%s MaxIdleConn:%d MaxOpenConn:%d Charset:%s}",
		c.Dialect, c.HostName, c.User, password, c.Port, c.Database,
		c.SSLMode, c.MaxIdleConn, c.MaxOpenConn, c.Charset)
}

// GoString implements fmt.GoStringer so that the %#v verb — which bypasses Stringer
// and would otherwise print the raw struct, password included — also redacts the
// password. It mirrors String's redaction so no formatting verb leaks the secret.
//
//nolint:gocritic // value receiver required so GoStringer is in the value's method set, matching String
func (c DBConfig) GoString() string {
	password := ""
	if c.Password != "" {
		password = redactedPassword
	}

	return fmt.Sprintf("sql.DBConfig{Dialect:%q, HostName:%q, User:%q, Password:%q, Port:%q, "+
		"Database:%q, SSLMode:%q, MaxIdleConn:%d, MaxOpenConn:%d, Charset:%q}",
		c.Dialect, c.HostName, c.User, password, c.Port, c.Database,
		c.SSLMode, c.MaxIdleConn, c.MaxOpenConn, c.Charset)
}

func setupSupabaseDefaults(dbConfig *DBConfig, configs config.Config, logger datasource.Logger) {
	if dbConfig.HostName == "" {
		projectRef := configs.Get("SUPABASE_PROJECT_REF")
		if projectRef != "" {
			dbConfig.HostName = fmt.Sprintf("db.%s.supabase.co", projectRef)
		}
	}

	if dbConfig.Database == "" {
		dbConfig.Database = dialectPostgres
	}

	if dbConfig.SSLMode != requireSSLMode {
		logger.Warnf("Supabase connections require SSL. Setting DB_SSL_MODE to 'require'")

		dbConfig.SSLMode = requireSSLMode // Enforce SSL mode for Supabase
	}

	if dbConfig.Port == strconv.Itoa(defaultDBPort) {
		dbConfig.Port = "5432"
	}
}

func NewSQL(configs config.Config, logger datasource.Logger, metrics Metrics) *DB {
	dbConfig := getDBConfig(configs)

	if dbConfig.Dialect == supabaseDialect {
		setupSupabaseDefaults(dbConfig, configs, logger)
	}

	if dbConfig.Dialect == "" {
		return nil
	}

	// if Hostname is not provided, we won't try to connect to DB
	if dbConfig.Dialect != sqlite && dbConfig.HostName == "" {
		logger.Errorf("connection to %s failed: host name is empty.", dbConfig.Dialect)
	}

	// Register MySQL TLS config if needed (BEFORE opening connection)
	if err := registerMySQLTLSConfig(dbConfig, logger); err != nil {
		if strings.Contains(strings.ToLower(dbConfig.SSLMode), "verify") {
			logger.Errorf("failed to register MySQL TLS config: %v", err)

			return nil
		}

		logger.Warnf("failed to register MySQL TLS config: %v", err)
	}

	logger.Debugf("generating database connection string for '%s'", dbConfig.Dialect)

	dbConnectionString, err := getDBConnectionString(dbConfig)
	if err != nil {
		logger.Error(errUnsupportedDialect)
		return nil
	}

	logger.Debugf("registering sql dialect '%s' for traces", dbConfig.Dialect)

	otelRegisteredDialect, err := registerOtel(dbConfig.Dialect, logger)
	if err != nil {
		logger.Errorf("could not register sql dialect '%s' for traces, error: %s", dbConfig.Dialect, err)
		return nil
	}

	database := &DB{config: dbConfig, logger: logger, metrics: metrics, stopSignal: make(chan struct{})}

	printConnectionSuccessLog("connecting", database.config, logger)

	database.DB, err = sql.Open(otelRegisteredDialect, dbConnectionString)
	if err != nil {
		printConnectionFailureLog("open connection with", database.config, database.logger, err)

		return database
	}

	return finalizeConnection(database)
}

// NewSQLFromDB wraps an already-opened *database/sql.DB in gofr's SQL datasource,
// adding query logging, metrics, health checks and the same background
// connection-retry/metrics goroutines used by NewSQL. Driver registration, DSN
// building and sql.Open are the caller's responsibility; this is the entry point
// for pluggable SQL datasources (for example GCP Cloud SQL with IAM auth) that are
// wired in via App.AddSQLDB.
//
// conf supplies the dialect plus the labels and pool sizing used in logs,
// metrics and health output. A nil conf or db returns nil.
//
// Because it accepts any opened *sql.DB, this is the provider-agnostic seam for
// managed-database authentication: GCP Cloud SQL passes a connector-backed handle,
// while AWS RDS/Aurora IAM and Azure Entra ID modules pass a sql.OpenDB(connector)
// whose connector mints a fresh short-lived token per connection. Each such
// provider lives in its own published module so its cloud SDK stays out of core.
// See gofr.dev/pkg/gofr/datasource/cloudsql for the reference implementation.
func NewSQLFromDB(db *sql.DB, conf *DBConfig, logger datasource.Logger, metrics Metrics) *DB {
	if db == nil || conf == nil {
		return nil
	}

	database := &DB{DB: db, config: conf, logger: logger, metrics: metrics, stopSignal: make(chan struct{})}

	printConnectionSuccessLog("connecting", database.config, logger)

	return finalizeConnection(database)
}

// NewSQLFromConnector wraps a driver.Connector in gofr's SQL datasource. It opens
// the connection with otelsql (so queries are traced, exactly as NewSQL does) and
// then shares the standard post-open path — query logging, metrics, health checks
// and the background retry/metrics goroutines — via NewSQLFromDB's finalize logic.
//
// This is the seam for pluggable, connector-based SQL providers where credentials
// or routing are owned by the connector rather than a DSN: GCP Cloud SQL with IAM
// auth supplies a Cloud SQL connector; AWS RDS/Aurora IAM and Azure Entra ID can
// supply a token-minting connector the same way. Each such provider lives in its
// own module and constructs only the driver.Connector, so its cloud SDK stays out
// of core; core does the wrapping here. See gofr.dev/pkg/gofr/datasource/cloudsql.
//
// conf supplies the dialect plus the labels and pool sizing used in logs, metrics
// and health output. cleanup, if non-nil, is run on Close to tear down resources
// database/sql does not own (for Cloud SQL, the connector's dialer and its
// background credential refresh). A nil connector returns nil.
func NewSQLFromConnector(connector driver.Connector, conf config.Config,
	logger datasource.Logger, metrics Metrics, cleanup func() error) *DB {
	if connector == nil {
		return nil
	}

	dbConfig := getDBConfig(conf)

	// The connector owns routing (host/port/TLS), so a port label is meaningless on
	// this path; clear it so connection logs don't print a dangling ':' after the host.
	dbConfig.Port = ""

	database := &DB{
		DB:         otelsql.OpenDB(connector),
		config:     dbConfig,
		logger:     logger,
		metrics:    metrics,
		stopSignal: make(chan struct{}),
		cleanup:    cleanup,
	}

	printConnectionSuccessLog("connecting", database.config, logger)

	return finalizeConnection(database)
}

// finalizeConnection applies pool limits, verifies connectivity and starts the
// background retry + metrics goroutines. Shared by NewSQL and NewSQLFromDB so both
// connection paths get identical post-open behavior.
func finalizeConnection(database *DB) *DB {
	// We are not setting idle connection timeout because we are checking for connection
	// every 10 seconds which would need a connection, moreover if connection expires it is
	// automatically closed by the database/sql package.
	database.DB.SetMaxIdleConns(database.config.MaxIdleConn)
	// We are not setting max open connection because any connection which is expired,
	// it is closed automatically.
	database.DB.SetMaxOpenConns(database.config.MaxOpenConn)

	database = pingToTestConnection(database)

	go retryConnection(database)

	go pushDBMetrics(database, database.metrics)

	return database
}

func registerOtel(dialect string, logger datasource.Logger) (string, error) {
	// Supabase and CockroachDB use the PostgreSQL driver, so we register them as the "postgres" dialect
	// to ensure compatibility with OpenTelemetry instrumentation.
	otelSupportedDialect := dialect

	if dialect == supabaseDialect || dialect == cockroachDB {
		logger.Debugf("using '%s' as an alias for '%s' for otel-sql registration", dialectPostgres, dialect)
		otelSupportedDialect = dialectPostgres
	}

	return otelsql.Register(otelSupportedDialect)
}

func pingToTestConnection(database *DB) *DB {
	if err := database.DB.PingContext(context.Background()); err != nil {
		printConnectionFailureLog("connect", database.config, database.logger, err)

		return database
	}

	printConnectionSuccessLog("connected", database.config, database.logger)

	return database
}

func retryConnection(database *DB) {
	const connRetryFrequencyInSeconds = 10

	retryDuration := connRetryFrequencyInSeconds * time.Second

	for {
		select {
		case <-database.stopSignal:
			return
		default:
		}

		if database.DB.PingContext(context.Background()) != nil {
			database.logger.Info("retrying SQL database connection")

			if !attemptReconnection(database, retryDuration) {
				return
			}
		}

		select {
		case <-time.After(retryDuration):
		case <-database.stopSignal:
			return
		}
	}
}

func attemptReconnection(database *DB, retryDuration time.Duration) bool {
	for {
		select {
		case <-database.stopSignal:
			return false
		default:
		}

		err := database.DB.PingContext(context.Background())
		if err == nil {
			printConnectionSuccessLog("connected", database.config, database.logger)

			return true
		}

		printConnectionFailureLog("connect", database.config, database.logger, err)

		select {
		case <-time.After(retryDuration):
		case <-database.stopSignal:
			return false
		}
	}
}

func getDBConfig(configs config.Config) *DBConfig {
	const (
		defaultMaxIdleConn = 2
		defaultMaxOpenConn = 0
	)

	// if the value of maxIdleConn is negative or 0, no idle connections are retained.
	maxIdleConn, err := strconv.Atoi(configs.Get("DB_MAX_IDLE_CONNECTION"))
	if err != nil {
		// setting the max open connection as the default which is being provided by default package
		maxIdleConn = defaultMaxIdleConn
	}

	// if the value of maxOpenConn is negative, it is treated as 0 by sql package.
	maxOpenConn, err := strconv.Atoi(configs.Get("DB_MAX_OPEN_CONNECTION"))
	if err != nil {
		// setting the max open connection as the default which is being provided by default
		// in this case there will be no limit for number of max open connections.
		maxOpenConn = defaultMaxOpenConn
	}

	return &DBConfig{
		Dialect:     configs.Get("DB_DIALECT"),
		HostName:    configs.Get("DB_HOST"),
		User:        configs.Get("DB_USER"),
		Password:    configs.Get("DB_PASSWORD"),
		Port:        configs.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
		Database:    configs.Get("DB_NAME"),
		MaxOpenConn: maxOpenConn,
		MaxIdleConn: maxIdleConn,
		// Supported for postgres, supabase, cockroachdb, and mysql
		SSLMode: configs.GetOrDefault("DB_SSL_MODE", sslModeDisable),
		Charset: configs.Get("DB_CHARSET"),
	}
}

func getDBConnectionString(dbConfig *DBConfig) (string, error) {
	switch dbConfig.Dialect {
	case dialectMysql:
		if dbConfig.Charset == "" {
			dbConfig.Charset = "utf8"
		}

		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&interpolateParams=true",
			dbConfig.User,
			dbConfig.Password,
			dbConfig.HostName,
			dbConfig.Port,
			dbConfig.Database,
			dbConfig.Charset,
		)

		if tlsParam := getMySQLTLSParam(dbConfig.SSLMode); tlsParam != "" {
			connStr = fmt.Sprintf("%s&%s", connStr, tlsParam)
		}

		return connStr, nil
	case dialectPostgres, supabaseDialect, cockroachDB:
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			dbConfig.HostName, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.Database, dbConfig.SSLMode), nil
	case sqlite:
		s := strings.TrimSuffix(dbConfig.Database, ".db")

		return fmt.Sprintf("file:%s.db", s), nil
	default:
		return "", errUnsupportedDialect
	}
}

func pushDBMetrics(database *DB, metrics Metrics) {
	const frequency = 10

	for {
		select {
		case <-database.stopSignal:
			return
		default:
		}

		if database.DB != nil {
			stats := database.DB.Stats()

			metrics.SetGauge("app_sql_open_connections", float64(stats.OpenConnections))
			metrics.SetGauge("app_sql_inUse_connections", float64(stats.InUse))

			select {
			case <-time.After(frequency * time.Second):
			case <-database.stopSignal:
				return
			}
		}
	}
}

func printConnectionSuccessLog(status string, dbconfig *DBConfig, logger datasource.Logger) {
	logFunc := logger.Infof
	if status != "connected" {
		logFunc = logger.Debugf
	}

	if dbconfig.Dialect == sqlite {
		logFunc("%s to '%s' database", status, dbconfig.Database)
	} else {
		logFunc("%s to '%s' user to '%s' database at '%s'", status, dbconfig.User, dbconfig.Database, hostEndpoint(dbconfig))
	}
}

func printConnectionFailureLog(action string, dbconfig *DBConfig, logger datasource.Logger, err error) {
	if dbconfig.Dialect == sqlite {
		logger.Errorf("could not %s database '%s', error: %v", action, dbconfig.Database, err)
	} else {
		logger.Errorf("could not %s '%s' user to '%s' database at '%s', error: %v",
			action, dbconfig.User, dbconfig.Database, hostEndpoint(dbconfig), err)
	}
}

// hostEndpoint renders the "host:port" connection endpoint for logs, omitting the
// ":port" suffix when no port is set. Connector-based datasources (e.g. Cloud SQL
// with IAM auth) leave Port empty because the connector owns routing, so this keeps
// their logs from printing a dangling ':' after the host.
func hostEndpoint(dbconfig *DBConfig) string {
	if dbconfig.Port == "" {
		return dbconfig.HostName
	}

	return dbconfig.HostName + ":" + dbconfig.Port
}

// getMySQLTLSParam converts the generic DB_SSL_MODE to MySQL-specific TLS parameter.
// For custom CA certificates, use DB_TLS_CA_CERT environment variable.
func getMySQLTLSParam(sslMode string) string {
	switch strings.ToLower(sslMode) {
	case sslModeDisable, "false":
		return "" // No TLS - insecure
	case "preferred":
		return "tls=preferred" // Try TLS, fallback to plain
	case requireSSLMode, "true":
		return tlsSkipVerify // TLS required but no cert validation
	case "skip-verify":
		return tlsSkipVerify // Explicit skip verification
	case sslModeVerifyCA, sslModeVerifyFull:
		return tlsCustom // Use custom TLS config with CA verification
	default:
		return "" // Default to no TLS
	}
}

// registerMySQLTLSConfig registers custom TLS configuration for MySQL if needed.
func registerMySQLTLSConfig(dbConfig *DBConfig, logger datasource.Logger) error {
	// Only for MySQL with verify-ca or verify-full
	if dbConfig.Dialect != dialectMysql {
		return nil
	}

	if !strings.Contains(strings.ToLower(dbConfig.SSLMode), "verify") {
		return nil // skip-verify doesn't need custom config
	}

	caCertPath := os.Getenv("DB_TLS_CA_CERT")
	if caCertPath == "" {
		logger.Warn("DB_SSL_MODE=verify-ca requires DB_TLS_CA_CERT. Falling back to system CA pool")

		// Use system CA pool
		tlsConfig := &tls.Config{
			ServerName: getServerName(dbConfig.HostName),
			MinVersion: tls.VersionTLS12,
		}

		return mysql.RegisterTLSConfig("custom", tlsConfig)
	}

	// Load custom CA certificate
	caCert, err := os.ReadFile(caCertPath) //nolint:gosec // caCertPath is an operator-supplied configuration path, not user input
	if err != nil {
		return fmt.Errorf("failed to read CA certificate from %s: %w", caCertPath, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return errFailedCACerts
	}

	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: dbConfig.HostName,
		MinVersion: tls.VersionTLS12,
	}

	// Optional: Support client certificates (mutual TLS)
	clientCertPath := os.Getenv("DB_TLS_CLIENT_CERT")
	clientKeyPath := os.Getenv("DB_TLS_CLIENT_KEY")

	if clientCertPath != "" && clientKeyPath != "" {
		clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}

		logger.Debug("loaded client certificate for mutual TLS")
	}

	return mysql.RegisterTLSConfig("custom", tlsConfig)
}

func getServerName(hostname string) string {
	// For localhost/127.0.0.1, use "localhost" explicitly
	if hostname == "127.0.0.1" || hostname == "::1" {
		return localhost
	}

	return hostname
}
