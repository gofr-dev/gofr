package sql

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/testutil"

	_ "github.com/lib/pq" // used for concrete implementation of the database driver.

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const defaultDBPort = 3306

var errUnsupportedDialect = fmt.Errorf("unsupported db dialect; supported dialects are - mysql, postgres")

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
	dbConfig := getDBConfig(configs)

	// if Hostname is not provided, we won't try to connect to DB
	if dbConfig.HostName == "" || dbConfig.Dialect == "" {
		return nil
	}

	dbConnectionString, err := getDBConnectionString(dbConfig)
	if err != nil {
		logger.Error(errUnsupportedDialect)
		return nil
	}

	db, err := sql.Open(dbConfig.Dialect, dbConnectionString)
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
	default:
		return "", errUnsupportedDialect
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

func NewSQLMocks(t *testing.T) (*DB, sqlmock.Sqlmock, *MockMetrics) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)

	return &DB{
		DB:      db,
		logger:  testutil.NewMockLogger(testutil.DEBUGLOG),
		config:  nil,
		metrics: mockMetrics,
	}, mock, mockMetrics
}
