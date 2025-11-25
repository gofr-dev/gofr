package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq" // used for concrete implementation of the database driver.
	_ "modernc.org/sqlite"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	sqlite         = "sqlite"
	cockroachDB    = "cockroachdb"
	defaultDBPort  = 3306
	requireSSLMode = "require"
)

var errUnsupportedDialect = fmt.Errorf(
	"unsupported db dialect; supported dialects are - mysql, postgres, supabase, sqlite, %s", cockroachDB)

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

	database := &DB{config: dbConfig, logger: logger, metrics: metrics}

	printConnectionSuccessLog("connecting", database.config, logger)

	database.DB, err = sql.Open(otelRegisteredDialect, dbConnectionString)
	if err != nil {
		printConnectionFailureLog("open connection with", database.config, database.logger, err)

		return database
	}

	// We are not setting idle connection timeout because we are checking for connection
	// every 10 seconds which would need a connection, moreover if connection expires it is
	// automatically closed by the database/sql package.
	database.DB.SetMaxIdleConns(dbConfig.MaxIdleConn)
	// We are not setting max open connection because any connection which is expired,
	// it is closed automatically.
	database.DB.SetMaxOpenConns(dbConfig.MaxOpenConn)

	database = pingToTestConnection(database)

	go retryConnection(database)

	go pushDBMetrics(database.DB, metrics)

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

	for {
		if database.DB.PingContext(context.Background()) != nil {
			database.logger.Info("retrying SQL database connection")

			for {
				err := database.DB.PingContext(context.Background())
				if err == nil {
					printConnectionSuccessLog("connected", database.config, database.logger)

					break
				}

				printConnectionFailureLog("connect", database.config, database.logger, err)

				time.Sleep(connRetryFrequencyInSeconds * time.Second)
			}
		}

		time.Sleep(connRetryFrequencyInSeconds * time.Second)
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
		// only for postgres
		SSLMode: configs.GetOrDefault("DB_SSL_MODE", "disable"),
		Charset: configs.Get("DB_CHARSET"),
	}
}

func getDBConnectionString(dbConfig *DBConfig) (string, error) {
	switch dbConfig.Dialect {
	case "mysql":
		if dbConfig.Charset == "" {
			dbConfig.Charset = "utf8"
		}

		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&interpolateParams=true",
			dbConfig.User,
			dbConfig.Password,
			dbConfig.HostName,
			dbConfig.Port,
			dbConfig.Database,
			dbConfig.Charset,
		), nil
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

func pushDBMetrics(db *sql.DB, metrics Metrics) {
	const frequency = 10

	for {
		if db != nil {
			stats := db.Stats()

			metrics.SetGauge("app_sql_open_connections", float64(stats.OpenConnections))
			metrics.SetGauge("app_sql_inUse_connections", float64(stats.InUse))

			time.Sleep(frequency * time.Second)
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
		logFunc("%s to '%s' user to '%s' database at '%s:%s'", status, dbconfig.User, dbconfig.Database, dbconfig.HostName, dbconfig.Port)
	}
}

func printConnectionFailureLog(action string, dbconfig *DBConfig, logger datasource.Logger, err error) {
	if dbconfig.Dialect == sqlite {
		logger.Errorf("could not %s database '%s', error: %v", action, dbconfig.Database, err)
	} else {
		logger.Errorf("could not %s '%s' user to '%s' database at '%s:%s', error: %v",
			action, dbconfig.User, dbconfig.Database, dbconfig.HostName, dbconfig.Port, err)
	}
}
