package datastore

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	gosqldriver "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	otelgorm "github.com/zopsmart/gorm-opentelemetry"

	// used for concrete implementation of the database driver.
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

const (
	invalidDialectErr  = "invalid dialect: supported dialects are - mysql, mssql, sqlite, postgres"
	pushMetricDuration = 100
	disable            = "disable"
)

type invalidDialect struct{}

func (i invalidDialect) Error() string {
	return invalidDialectErr
}

// DBConfig stores the config variables required to connect to a supported database
type DBConfig struct {
	HostName string
	Username string
	Password string
	Database string
	Port     string
	Dialect  string // supported dialects are - mysql, mssql, sqlite, postgres
	// postgres and mysql related config only, accepts disable, allow, prefer, require,
	// verify-ca and verify-full; default is disable
	SSL               string
	ORM               string
	CertificateFile   string
	KeyFile           string
	ConnRetryDuration int
	MaxOpenConn       int
	MaxIdleConn       int
	MaxConnLife       int
	CACertificateFile string
}

// GORMClient stores a GORM database client along with logger and configs to connect to GORM DB.
type GORMClient struct {
	*gorm.DB
	logger log.Logger
	config *DBConfig
}

// SQLTx represents a SQL Transaction.
type SQLTx struct {
	*sql.Tx
	logger log.Logger
	config *DBConfig
}

// SQLClient stores a SQL database client along with logger and configs to connect to SQL DB.
type SQLClient struct {
	*sql.DB
	logger log.Logger
	config *DBConfig
}

//nolint:gochecknoglobals // sqlStats has to be a global variable for prometheus
var (
	sqlStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "zs_sql_stats",
		Help:    "Histogram for SQL",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host", "database"})

	sqlOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_sql_open_connections",
		Help: "Gauge for sql open connections",
	}, []string{"database", "host"})

	sqlIdle = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_sql_idle_connections",
		Help: "Gauge for sql idle connections",
	}, []string{"database", "host"})

	sqlInUse = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_sql_inUse_connections",
		Help: "Gauge for sql InUse connections",
	}, []string{"database", "host"})

	_ = prometheus.Register(sqlStats)
	_ = prometheus.Register(sqlOpen)
	_ = prometheus.Register(sqlIdle)
	_ = prometheus.Register(sqlInUse)
)

// NewORM returns a new ORM object if the config is correct, otherwise it returns the error thrown
func NewORM(cfg *DBConfig) (GORMClient, error) {
	validDialects := map[string]bool{
		"mysql":    true,
		"mssql":    true,
		"postgres": true,
		"sqlite":   true,
	}

	if _, ok := validDialects[cfg.Dialect]; !ok {
		return GORMClient{config: cfg}, invalidDialect{}
	}

	connectionStr := formConnectionStr(cfg)

	var (
		db  *gorm.DB
		err error
		d   gorm.Dialector
	)

	driverName := registerDialect(cfg.Dialect)

	switch cfg.Dialect {
	case mySQL:
		d, err = NewMySQLDialector(cfg, connectionStr, driverName)
		if err != nil {
			return GORMClient{}, err
		}
	case pgSQL:
		d = postgres.New(postgres.Config{DriverName: driverName, DSN: connectionStr})
	case "sqlite":
		d = sqlite.Dialector{DriverName: driverName, DSN: connectionStr}
	case "mssql":
		// driverName is not added to the config. Currently, it breaks migrations for sqlserver.
		d = sqlserver.New(sqlserver.Config{DSN: connectionStr})
	default:
		return GORMClient{config: cfg}, errors.DB{}
	}

	db, err = dbConnection(d)
	if err != nil {
		// Check for URL error, to avoid logging of password in case of postgres and mssql.
		// issue is from external packages(postgres and mssql), once that is upgraded, this check can be removed.
		urlErr, ok := err.(*url.Error)
		if ok {
			hashedConfig := *cfg
			hashedConfig.Username = "########"
			hashedConfig.Password = "********"
			urlErr.URL = formConnectionStr(&hashedConfig)

			return GORMClient{config: cfg}, urlErr
		}

		return GORMClient{config: cfg}, err
	}

	sqlDB, err := db.DB()
	if err == nil {
		setPoolConnConfigs(cfg, sqlDB)

		go pushConnMetrics(cfg.Database, cfg.HostName, sqlDB)
	}

	return GORMClient{DB: db, config: cfg}, err
}

// NewORMFromEnv fetches the config from environment variables and returns a new ORM object if the config was
// correct, otherwise returns the thrown error
// Deprecated: Instead use datastore.NewORM
func NewORMFromEnv() (GORMClient, error) {
	// pushing deprecated feature count to prometheus
	middleware.PushDeprecatedFeature("NewORMFromEnv")

	return NewORM(dbConfigFromEnv())
}

// SQLXClient stores a SQLX database client along with logger to connect to SQL DB.
type SQLXClient struct {
	*sqlx.DB
	config *DBConfig
}

// NewSQLX returns a new sqlx.DB object if the given config is correct, otherwise throws an error
func NewSQLX(cfg *DBConfig) (SQLXClient, error) {
	connectionStr := formConnectionStr(cfg)

	db, err := sqlx.Connect(cfg.Dialect, connectionStr)
	if err != nil {
		return SQLXClient{config: cfg}, err
	}

	setPoolConnConfigs(cfg, db.DB)

	go pushConnMetrics(cfg.Database, cfg.HostName, db.DB)

	return SQLXClient{DB: db, config: cfg}, nil
}

// dbConfigFromEnv fetches the DBConfig from environment
func dbConfigFromEnv() *DBConfig {
	return &DBConfig{
		HostName:          os.Getenv("DB_HOST"),
		Username:          os.Getenv("DB_USER"),
		Password:          os.Getenv("DB_PASSWORD"),
		Database:          os.Getenv("DB_NAME"),
		Port:              os.Getenv("DB_PORT"),
		Dialect:           os.Getenv("DB_DIALECT"),
		SSL:               os.Getenv("DB_SSL"),
		CertificateFile:   os.Getenv("DB_CERTIFICATE_FILE"),
		KeyFile:           os.Getenv("DB_KEY_FILE"),
		CACertificateFile: os.Getenv("DB_CA_CERTIFICATE_FILE"),
	}
}

// formConnection string forms a DB connection string based on the DB Dialect and the given configuration
func formConnectionStr(cfg *DBConfig) string {
	var (
		escapedUsrName  = queryEscape(cfg.Username)
		escapedPassword = queryEscape(cfg.Password)
	)

	switch cfg.Dialect {
	case "postgres":
		ssl := strings.ToLower(cfg.SSL)
		if ssl == "" {
			cfg.SSL = disable
		}

		return fmt.Sprintf("postgres://%v@%v:%v/%v?password=%v&sslmode=%v&sslcert=%v&sslkey=%v",
			escapedUsrName, cfg.HostName, cfg.Port, cfg.Database, escapedPassword, cfg.SSL, cfg.CertificateFile, cfg.KeyFile)
	case "mssql":
		return fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			escapedUsrName, escapedPassword, cfg.HostName, cfg.Port, cfg.Database)
	default:
		ssl := strings.ToLower(cfg.SSL)
		if ssl == "" || ssl == disable {
			return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local",
				escapedUsrName, cfg.Password, cfg.HostName, cfg.Port, cfg.Database)
		}
		// Indicate that a custom TLS configuration should be used.
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?tls=custom&charset=utf8&parseTime=True&loc=Local",
			escapedUsrName, cfg.Password, cfg.HostName, cfg.Port, cfg.Database)
	}
}

func queryEscape(value string) string {
	// QueryUnescape will return an error if any % is not followed by two hexadecimal digits.
	unescapedVal, err := url.QueryUnescape(value)
	if err != nil {
		unescapedVal = value
	}

	// Escaping the special characters.
	return url.QueryEscape(unescapedVal)
}

func (c GORMClient) logError(err error) {
	if c.logger != nil {
		c.logger.Errorf("%v", err)
	}
}

// HealthCheck pings the sql instance in gorm. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN
func (c GORMClient) HealthCheck() types.Health {
	resp := types.Health{
		Name:     SQLStore,
		Status:   pkg.StatusDown,
		Host:     c.config.HostName,
		Database: c.config.Database,
	}

	// The following check is for the condition when the connection to SQL has not been made during initialization
	if c.DB == nil {
		c.logError(errors.HealthCheckFailed{Dependency: SQLStore, Reason: "sql not initialized"})
		return resp
	}

	sqlDB, err := c.DB.DB()
	if err != nil {
		c.logError(errors.HealthCheckFailed{Dependency: SQLStore, Err: err})
		return resp
	}

	err = sqlDB.Ping()
	if err != nil {
		c.logError(errors.HealthCheckFailed{Dependency: SQLStore, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp
	resp.Details = sqlDB.Stats()

	return resp
}

// HealthCheck pings the sqlx instance in gorm. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN
func (c SQLXClient) HealthCheck() types.Health {
	resp := types.Health{
		Name:     SQLStore,
		Status:   pkg.StatusDown,
		Host:     c.config.HostName,
		Database: c.config.Database,
	}
	// The following check is for the condition when the connection to SQLX has not been made during initialization
	if c.DB == nil {
		return resp
	}

	err := c.DB.Ping()
	if err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp
	resp.Details = c.DB.Stats()

	return resp
}

// dbConnection will establish a database connection based on the gorm.Dialector passed and returns a gorm.DB instance
func dbConnection(d gorm.Dialector) (db *gorm.DB, err error) {
	// Silent the default gorm logger. Else redundant error logs will be logged.
	db, err = gorm.Open(d, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		return
	}

	opts := otelgorm.WithTracerProvider(otel.GetTracerProvider())
	plugin := otelgorm.NewPlugin(opts)

	_ = db.Use(plugin)

	return
}

// registerDialect registers the dialect to instrument the database/sql pkg and returns driverName based on the db Dialect.
func registerDialect(dialect string) (driverName string) {
	if dialect == pgSQL {
		driverName, _ = otelsql.Register(dialect, semconv.DBSystemPostgreSQL.Value.AsString())
	} else {
		driverName, _ = otelsql.Register(dialect, dialect)
	}

	return
}

// pushConnMetrics pushes SQL connection pool values to metrics for every 100 millisecond
func pushConnMetrics(database, hostname string, db *sql.DB) {
	for {
		stats := db.Stats()
		sqlOpen.WithLabelValues(database, hostname).Set(float64(stats.OpenConnections))
		sqlIdle.WithLabelValues(database, hostname).Set(float64(stats.Idle))
		sqlInUse.WithLabelValues(database, hostname).Set(float64(stats.InUse))
		time.Sleep(pushMetricDuration * time.Millisecond)
	}
}

// setPoolConnConfigs sets the SQL connection pool values to database/sql pkg
func setPoolConnConfigs(cfg *DBConfig, db *sql.DB) {
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetConnMaxLifetime(time.Duration(cfg.MaxConnLife) * time.Second)
}

// createSSLConfig generates a custom TLS config for secure database connections.
func createSSLConfig(caCertPath, clientCertPath, clientKeyPath string) (*tls.Config, error) {
	rootCertPool := x509.NewCertPool()

	pem, err := os.ReadFile(caCertPath)
	if err != nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	clientCert := make([]tls.Certificate, 0, 1)

	certs, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	clientCert = append(clientCert, certs)

	return &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCert,
		//nolint:gosec // cannot keep InsecureSkipVerify as false as we are using self signed certificates
		InsecureSkipVerify: true, // needed for self signed certs
		MinVersion:         tls.VersionTLS12,
	}, nil
}

// NewMySQLDialector creates a new GORM Dialector for MySQL database based on the provided configurations.
func NewMySQLDialector(cfg *DBConfig, connectionStr, driverName string) (gorm.Dialector, error) {
	ssl := strings.ToLower(cfg.SSL)
	if ssl != "" && ssl != disable {
		sslConf, err := createSSLConfig(cfg.CACertificateFile, cfg.CertificateFile, cfg.KeyFile)
		if err != nil {
			return nil, err
		}

		// It registers a custom tls.Config to be used with sql.Open.
		err = gosqldriver.RegisterTLSConfig("custom", sslConf)
		if err != nil {
			return nil, err
		}

		// Opens a new SQL connection using the "mysql" driver with the provided connection string.
		open, err := sql.Open("mysql", connectionStr)
		if err != nil {
			return nil, err
		}

		// Returns a new GORM Dialector configured for MySQL with the custom SSL connection.
		return mysql.New(mysql.Config{Conn: open}), nil
	}

	// Returns a new GORM Dialector configured for MySQL with the provided driver name and connection string when SSL is not enabled.
	return mysql.New(mysql.Config{DriverName: driverName, DSN: connectionStr}), nil
}
