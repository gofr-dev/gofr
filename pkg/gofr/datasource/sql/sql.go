package sql

import (
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
	sqlite        = "sqlite"
	defaultDBPort = 3306
)

var errUnsupportedDialect = fmt.Errorf("unsupported db dialect; supported dialects are - mysql, postgres, sqlite")

// DBConfig has those members which are necessary variables while connecting to database.
type DBConfig struct {
	Dialect  string
	HostName string
	User     string
	Password string
	Port     string
	Database string
}

func NewSQL(configs config.Config, logger datasource.Logger, metrics Metrics) *DB {
	logger.Debugf("reading database configurations from config file")

	dbConfig := getDBConfig(configs)

	// if Hostname is not provided, we won't try to connect to DB
	if dbConfig.Dialect != sqlite && dbConfig.HostName == "" {
		logger.Debugf("not connecting to database as database configurations aren't available")
		return nil
	}

	logger.Debugf("generating database connection string for '%s'", dbConfig.Dialect)

	dbConnectionString, err := getDBConnectionString(dbConfig)
	if err != nil {
		logger.Error(errUnsupportedDialect)
		return nil
	}

	logger.Debugf("registering sql dialect '%s' for traces", dbConfig.Dialect)

	otelRegisteredDialect, err := otelsql.Register(dbConfig.Dialect)
	if err != nil {
		logger.Errorf("could not register sql dialect '%s' for traces, error: %s", dbConfig.Dialect, err)
		return nil
	}

	database := &DB{config: dbConfig, logger: logger, metrics: metrics}

	logger.Debugf("connecting to '%s' user to '%s' database at '%s:%s'", database.config.User,
		database.config.Database, database.config.HostName, database.config.Port)

	database.DB, err = sql.Open(otelRegisteredDialect, dbConnectionString)
	if err != nil {
		database.logger.Errorf("could not open connection with '%s' user to '%s' database at '%s:%s', error: %v",
			database.config.User, database.config.Database, database.config.HostName, database.config.Port, err)

		return database
	}

	database = pingToTestConnection(database)

	go retryConnection(database)

	go pushDBMetrics(database.DB, metrics)

	return database
}

func pingToTestConnection(database *DB) *DB {
	if err := database.DB.Ping(); err != nil {
		database.logger.Errorf("could not connect with '%s' user to '%s' database at '%s:%s', error: %v",
			database.config.User, database.config.Database, database.config.HostName, database.config.Port, err)

		return database
	}

	database.logger.Logf("connected to '%s' database at '%s:%s'", database.config.Database,
		database.config.HostName, database.config.Port)

	return database
}

func retryConnection(database *DB) {
	const connRetryFrequencyInSeconds = 10

	for {
		if database.DB.Ping() != nil {
			database.logger.Log("retrying SQL database connection")

			for {
				if err := database.DB.Ping(); err != nil {
					database.logger.Debugf("could not connect with '%s' user to '%s' database at '%s:%s', error: %v",
						database.config.User, database.config.Database, database.config.HostName, database.config.Port, err)

					time.Sleep(connRetryFrequencyInSeconds * time.Second)
				} else {
					database.logger.Logf("connected to '%s' database at '%s:%s'", database.config.Database,
						database.config.HostName, database.config.Port)

					break
				}
			}
		}

		time.Sleep(connRetryFrequencyInSeconds * time.Second)
	}
}

func getDBConfig(configs config.Config) *DBConfig {
	return &DBConfig{
		Dialect:  configs.Get("DB_DIALECT"),
		HostName: configs.Get("DB_HOST"),
		User:     configs.Get("DB_USER"),
		Password: configs.Get("DB_PASSWORD"),
		Port:     configs.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
		Database: configs.Get("DB_NAME"),
	}
}

func getDBConnectionString(dbConfig *DBConfig) (string, error) {
	switch dbConfig.Dialect {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local&interpolateParams=true",
			dbConfig.User,
			dbConfig.Password,
			dbConfig.HostName,
			dbConfig.Port,
			dbConfig.Database,
		), nil
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbConfig.HostName, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.Database), nil
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
