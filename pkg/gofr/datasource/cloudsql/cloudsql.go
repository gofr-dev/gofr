// Package cloudsql provides a Google Cloud SQL (Postgres and MySQL) connector for
// GoFr that authenticates with IAM database authentication via the Cloud SQL Go
// Connector. It builds only a database/sql driver.Connector and hands it to GoFr —
// it does not import GoFr core, so the GCP SDK stays out of any app that does not
// use it. GoFr's App.AddSQLDB opens the connector through GoFr's standard SQL
// datasource, so logging, metrics, health checks and transactions behave exactly as
// for any other GoFr SQL connection.
//
// A single configuration switch, DB_IAM_AUTH, selects the connection mode:
//
//   - DB_IAM_AUTH=true  → connect through the Cloud SQL connector using IAM auth.
//     Credentials resolve via Application Default Credentials (which supports
//     Workload Identity Federation), so no static password and no Cloud SQL Auth
//     Proxy sidecar are required.
//   - otherwise         → Connect returns a nil connector, so App.AddSQLDB keeps
//     GoFr's standard env-configured SQL datasource (host:port with
//     username/password), exactly as gofr.New() would on its own.
//
// Because the mode is chosen from configuration, application code is identical in
// both environments — there is no conditional to write:
//
//	app := gofr.New()
//	app.AddSQLDB(cloudsql.New(app.Config))
//	app.Run()
//
// Set username/password locally and DB_IAM_AUTH=true on GCP; the code does not
// change. In both cases app.SQL()/ctx.SQL and all of GoFr's SQL logging, metrics,
// health checks and transactions behave identically, because GoFr always wraps the
// connection in its standard SQL datasource.
package cloudsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// netSeq makes every MySQL dial-context registration name unique so multiple
// connectors never collide on a registration. go-sql-driver has no unregister, so
// the name must be unique per connector rather than reused.
//
//nolint:gochecknoglobals // process-wide counter for unique dial-context registration names
var netSeq atomic.Uint64

var (
	errUnsupportedDialect = errors.New("cloudsql: unsupported dialect; supported dialects are postgres and mysql")
	errMissingInstance    = errors.New(
		"cloudsql: DB_HOST (instance connection name 'project:region:instance') is required for IAM auth")
)

// Config is the read-only configuration accessor cloudsql needs. It is defined
// locally — rather than importing GoFr's config package — so this module never
// depends on GoFr core. GoFr's app.Config satisfies it, so the one-liner
// app.AddSQLDB(cloudsql.New(app.Config)) compiles unchanged.
type Config interface {
	Get(key string) string
	GetOrDefault(key, defaultValue string) string
}

// Connector builds a database/sql driver.Connector for a Cloud SQL instance from
// configuration. It is handed to GoFr's App.AddSQLDB, which calls Connect and wraps
// the result in GoFr's standard SQL datasource. Keeping this module's output a plain
// driver.Connector is what keeps GoFr core (and every other app) free of the GCP SDK.
type Connector struct {
	conf Config
}

// New returns a Cloud SQL Connector that reads its configuration (the DB_* keys)
// from conf. No connection is established here; App.AddSQLDB calls Connect:
//
//	app.AddSQLDB(cloudsql.New(app.Config))
func New(conf Config) *Connector {
	return &Connector{conf: conf}
}

// Connect builds the driver.Connector for IAM auth and a cleanup that tears down the
// Cloud SQL dialer (stopping its background credential refresh). When IAM auth is not
// requested it returns a nil connector and nil cleanup, signaling App.AddSQLDB to
// keep GoFr's standard env-configured SQL connection — so the same code and the same
// AddSQLDB call work locally and on GCP, with only configuration changing.
//
// Connect is one-shot per Connector. The MySQL path registers a process-global
// dial-context under a unique name (database/sql/go-sql-driver have no unregister),
// so a Connector should be connected once over its lifetime rather than reconnected
// in a loop; the Postgres path registers nothing global.
func (c *Connector) Connect() (driver.Connector, func() error, error) {
	if !iamRequested(c.conf) {
		return nil, nil, nil
	}

	s := parseSettings(c.conf)

	if s.dialect == "" {
		return nil, nil, errUnsupportedDialect
	}

	if s.instanceConnectionName == "" {
		return nil, nil, errMissingInstance
	}

	switch s.dialect {
	case dialectPostgres:
		return postgresConnector(&s)
	case dialectMySQL:
		return mysqlConnector(&s)
	default:
		return nil, nil, errUnsupportedDialect
	}
}

// postgresConnector builds a pgx driver.Connector that dials through the Cloud SQL
// connector. It mirrors what cloudsqlconn's pgxv5 driver does internally (parse a
// config, route its DialFunc through the dialer) but yields a connector for
// sql.OpenDB instead of registering a process-global database/sql driver.
func postgresConnector(s *settings) (driver.Connector, func() error, error) {
	dialer, err := cloudsqlconn.NewDialer(context.Background(), s.connectorOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudsql: create dialer: %w", err)
	}

	cfg, err := pgx.ParseConfig(s.postgresDSN())
	if err != nil {
		_ = dialer.Close()

		return nil, nil, fmt.Errorf("cloudsql: parse postgres config: %w", err)
	}

	icn := s.instanceConnectionName
	cfg.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.Dial(ctx, icn)
	}

	return stdlib.GetConnector(*cfg), dialer.Close, nil
}

// mysqlConnector builds a go-sql-driver driver.Connector that dials through the
// Cloud SQL connector. go-sql-driver dials by a registered network name, so a unique
// dial-context routing through the dialer is registered per connector.
func mysqlConnector(s *settings) (driver.Connector, func() error, error) {
	dialer, err := cloudsqlconn.NewDialer(context.Background(), s.connectorOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudsql: create dialer: %w", err)
	}

	icn := s.instanceConnectionName

	netName := fmt.Sprintf("cloudsql-mysql-%d", netSeq.Add(1))
	mysql.RegisterDialContext(netName, func(ctx context.Context, _ string) (net.Conn, error) {
		return dialer.Dial(ctx, icn)
	})

	cfg := mysql.NewConfig()
	cfg.User = s.user
	cfg.Net = netName
	cfg.Addr = icn
	cfg.DBName = s.database
	cfg.ParseTime = true

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		_ = dialer.Close()

		return nil, nil, fmt.Errorf("cloudsql: create mysql connector: %w", err)
	}

	return connector, dialer.Close, nil
}
