package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
	gofrErr "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

type ClickHouseConfig struct {
	Host              string
	Port              string
	Username          string
	Password          string
	Database          string
	ConnRetryDuration int
	MaxOpenConn       int
	MaxIdleConn       int
	MaxConnLife       int
}

type ClickHouseDB struct {
	driver.Conn
	config *ClickHouseConfig
	logger log.Logger
}

//nolint:gochecknoglobals // clickhouseStats has to be a global variable for prometheus
var (
	clickhouseStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "zs_clickhouse_stats",
		Help:    "Histogram for CLICKHOUSE",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host", "database"})

	clickhouseOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_clickhouse_open_connections",
		Help: "Gauge for clickhouse open connections",
	}, []string{"database", "host"})

	clickhouseIdle = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_clickhouse_idle_connections",
		Help: "Gauge for clickhouse idle connections",
	}, []string{"database", "host"})

	_ = prometheus.Register(clickhouseStats)
	_ = prometheus.Register(clickhouseOpen)
	_ = prometheus.Register(clickhouseIdle)
)

func GetNewClickHouseDB(logger log.Logger, config *ClickHouseConfig) (ClickHouseDB, error) {
	connect, err := clickhouse.Open(&clickhouse.Options{
		Addr:            []string{fmt.Sprintf("%s:%s", config.Host, config.Port)},
		Auth:            clickhouse.Auth{Database: config.Database, Username: config.Username, Password: config.Password},
		MaxOpenConns:    config.MaxOpenConn,
		MaxIdleConns:    config.MaxIdleConn,
		ConnMaxLifetime: time.Duration(config.MaxConnLife),
	})
	if err != nil {
		return ClickHouseDB{}, err
	}

	if err := connect.Ping(context.Background()); err != nil {
		var exception *clickhouse.Exception
		if errors.As(err, &exception) {
			logger.Errorf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}

		return ClickHouseDB{}, err
	}

	go pushClickhouseConnMetrics(config.Database, config.Host, connect)

	db := ClickHouseDB{Conn: connect, config: config}

	return db, nil
}

// pushConnMetrics pushes CLICKHOUSE connection pool values to metrics for every 100 millisecond
func pushClickhouseConnMetrics(database, hostname string, db driver.Conn) {
	for {
		stats := db.Stats()
		clickhouseOpen.WithLabelValues(database, hostname).Set(float64(stats.Open))
		clickhouseIdle.WithLabelValues(database, hostname).Set(float64(stats.Idle))
		time.Sleep(pushMetricDuration * time.Millisecond)
	}
}

// HealthCheck pings the clickHouse instance in gorm. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN
func (c ClickHouseDB) HealthCheck() types.Health {
	resp := types.Health{
		Name:   ClickHouse,
		Status: pkg.StatusDown,
		Host:   c.config.Host,
	}
	// The following check is for the condition when the connection to CLICKHOUSE has not been made during initialization
	if c.Conn == nil {
		return resp
	}

	err := c.Conn.Ping(context.Background())
	if err != nil {
		c.logger.Error(gofrErr.HealthCheckFailed{Dependency: ClickHouse, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp
	resp.Details = c.Conn.Stats()

	return resp
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
func (c *ClickHouseDB) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	if c == nil || c.Conn == nil {
		return nil, gofrErr.ClickhouseNotInitialized
	}

	begin := time.Now()
	rows, err := c.Conn.Query(ctx, query, args...)

	c.monitorQuery(begin, query)

	return rows, err
}

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
func (c *ClickHouseDB) Exec(ctx context.Context, query string, args ...interface{}) error {
	if c == nil || c.Conn == nil {
		return gofrErr.ClickhouseNotInitialized
	}

	begin := time.Now()

	err := c.Conn.Exec(ctx, query, args...)

	c.monitorQuery(begin, query)

	return err
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always returns a non-nil value. Errors are deferred until Row's Scan method is called.
func (c *ClickHouseDB) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	begin := time.Now()

	row := c.Conn.QueryRow(ctx, query, args...)

	c.monitorQuery(begin, query)

	return row
}

func (c *ClickHouseDB) monitorQuery(begin time.Time, query string) {
	if c == nil || c.Conn == nil {
		return
	}

	var hostName, dbName string

	dur := time.Since(begin).Seconds()

	if c.config != nil {
		hostName = c.config.Host
		dbName = c.config.Database
	}

	// push stats to prometheus
	clickhouseStats.WithLabelValues(checkQueryOperation(query), hostName, dbName).Observe(dur)

	ql := QueryLogger{
		Query:     []string{query},
		DataStore: ClickHouse,
	}

	// log the query
	if c.logger != nil {
		ql.Duration = time.Since(begin).Microseconds()
		c.logger.Debug(ql)
	}
}
