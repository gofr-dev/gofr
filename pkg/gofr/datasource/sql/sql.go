package sql

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const defaultDBPort = 3306

// DBConfig has those members which are necessary variables while connecting to database.
type DBConfig struct {
	HostName string
	User     string
	Password string
	Port     string
	Database string
}

func NewSQL(configs config.Config, logger datasource.Logger, metrics Metrics) *DB {
	dbConfig := getDBConfig(configs)

	// if Hostname is not provided, we won't try to connect to DB
	if dbConfig.HostName == "" {
		return nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local&interpolateParams=true",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.HostName,
		dbConfig.Port,
		dbConfig.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Errorf("could not connect with '%s' user to database '%s:%s'  error: %v",
			dbConfig.User, dbConfig.HostName, dbConfig.Port, err)

		return &DB{config: dbConfig, metrics: metrics}
	}

	if err := db.Ping(); err != nil {
		logger.Errorf("could not connect with '%s' user to database '%s:%s'  error: %v",
			dbConfig.User, dbConfig.HostName, dbConfig.Port, err)

		return &DB{config: dbConfig, metrics: metrics, logger: logger}
	}

	logger.Logf("connected to '%s' database at %s:%s", dbConfig.Database, dbConfig.HostName, dbConfig.Port)

	go pushDBMetrics(db, metrics)

	return &DB{DB: db, config: dbConfig, logger: logger, metrics: metrics}
}

func getDBConfig(configs config.Config) *DBConfig {
	return &DBConfig{
		HostName: configs.Get("DB_HOST"),
		User:     configs.Get("DB_USER"),
		Password: configs.Get("DB_PASSWORD"),
		Port:     configs.GetOrDefault("DB_PORT", strconv.Itoa(defaultDBPort)),
		Database: configs.Get("DB_NAME"),
	}
}

func pushDBMetrics(db *sql.DB, metrics Metrics) {
	const frequency = 10

	for {
		stats := db.Stats()

		metrics.SetGauge("app_sql_open_connections", float64(stats.OpenConnections))
		metrics.SetGauge("app_sql_inUse_connections", float64(stats.InUse))

		time.Sleep(frequency * time.Second)
	}
}
