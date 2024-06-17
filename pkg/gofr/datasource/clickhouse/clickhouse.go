package clickhouse

import (
	"context"
	"crypto/tls"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"strings"
	"time"
)

type Config struct {
	Hosts    string
	Username string
	Password string
	Database string
}

type client struct {
	conn    driver.Conn
	config  Config
	logger  Logger
	metrics Metrics
}

func New(config Config) *client {
	return &client{config: config}
}

func (c *client) Exec(ctx context.Context, query string, args ...any) error {
	defer c.logQueryAndSendMetrics(time.Now(), "Exec", query, args...)

	return c.conn.Exec(ctx, query, args...)
}

func (c *client) Select(ctx context.Context, dest any, query string, args ...any) error {
	defer c.logQueryAndSendMetrics(time.Now(), "Select", query, args...)

	return c.conn.Select(ctx, dest, query, args...)
}

func (c *client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	defer c.logQueryAndSendMetrics(time.Now(), "AsyncInsert", query, args...)

	return c.conn.AsyncInsert(ctx, query, wait, args...)
}

// UseLogger sets the logger for the Clickhouse client.
func (c *client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Clickhouse client.
func (c *client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Connect establishes a connection to Clickhouse and registers metrics using the provided configuration when the client was Created.
func (c *client) Connect() {
	var err error

	clickHouseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in milliseconds.", clickHouseBuckets...)

	c.metrics.NewGauge("app_clickhouse_open_connections", "Number of open Clickhouse connections.")
	c.metrics.NewGauge("app_clickhouse_idle_connections", "Number of idle Clickhouse connections.")

	addresses := strings.Split(c.config.Hosts, ",")

	ctx := context.Background()
	c.conn, err = clickhouse.Open(&clickhouse.Options{
		Addr: addresses,
		Auth: clickhouse.Auth{
			Database: c.config.Database,
			Username: c.config.Username,
			Password: c.config.Password,
		},
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	if err != nil {
		c.logger.Errorf("error while connecting to clickhouse %v", err)
		return
	}

	if err = c.conn.Ping(ctx); err != nil {
		c.logger.Errorf("ping failed with error %v", err)

		return
	}

	go pushDBMetrics(c.conn, c.metrics)
}

func pushDBMetrics(conn driver.Conn, metrics Metrics) {
	const frequency = 10

	for {
		if conn != nil {
			stats := conn.Stats()

			metrics.SetGauge("app_clickhouse_open_connections", float64(stats.Open))
			metrics.SetGauge("app_clickhouse_idle_connections", float64(stats.Idle))

			time.Sleep(frequency * time.Second)
		}
	}
}

func (c *client) logQueryAndSendMetrics(start time.Time, methodType, query string, args ...interface{}) {
	duration := time.Since(start).Milliseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	c.metrics.RecordHistogram(context.Background(), "app_clickhouse_stats", float64(duration), "address", c.config.Hosts,
		"database", c.config.Database, "type", getOperationType(query))
}

func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")

	return words[0]
}
