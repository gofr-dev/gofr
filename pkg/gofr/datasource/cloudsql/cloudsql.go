// Package cloudsql provides a GoFr SQL datasource for Google Cloud SQL (Postgres
// and MySQL). It is built on top of GoFr's standard SQL datasource — it does not
// reimplement any SQL behavior — and adds IAM database authentication via the
// Cloud SQL Go Connector.
//
// A single configuration switch, DB_IAM_AUTH, selects the connection mode:
//
//   - DB_IAM_AUTH=true  → connect through the Cloud SQL connector using IAM auth.
//     Credentials resolve via Application Default Credentials (which supports
//     Workload Identity Federation), so no static password and no Cloud SQL Auth
//     Proxy sidecar are required.
//   - otherwise         → use GoFr's standard SQL datasource (host:port with
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
// health checks and transactions behave identically, because the underlying
// connection is always GoFr's standard SQL wrapper.
package cloudsql

import (
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"

	cloudmysql "cloud.google.com/go/cloudsqlconn/mysql/mysql"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/XSAM/otelsql"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

// driverSeq makes every registered connector driver name unique so multiple client
// instances (or repeated Connect calls) never collide on a driver registration.
//
//nolint:gochecknoglobals // process-wide counter for unique driver registration names
var driverSeq atomic.Uint64

var errUnsupportedDialect = errors.New("cloudsql: unsupported dialect; supported dialects are postgres and mysql")

// client is a GoFr SQL datasource backed by Cloud SQL. It is added to an
// application with App.AddSQLDB, which injects the logger and metrics and then
// calls Connect. The embedded *gofrSQL.DB — GoFr's standard SQL datasource — is
// populated by Connect and promotes the full container.DB interface, so the
// datasource is used exactly like any other GoFr SQL connection.
//
// It is unexported on purpose: callers only ever hold the container.DB returned by
// New and hand it to App.AddSQLDB, which drives the UseLogger/UseMetrics/Connect
// lifecycle. Keeping the type private keeps the module's public surface to just New.
type client struct {
	*gofrSQL.DB // GoFr's standard SQL wrapper; promotes the container.DB methods

	conf    config.Config
	logger  datasource.Logger
	metrics gofrSQL.Metrics
	cleanup func() error // tears down the Cloud SQL connector registration (IAM path only)
}

// New returns a Cloud SQL datasource that reads its configuration (the DB_* keys)
// from conf. The connection is not established until Connect is called, which
// App.AddSQLDB does automatically after wiring the logger and metrics — so the
// return value is only meaningful once passed to App.AddSQLDB:
//
//	app.AddSQLDB(cloudsql.New(app.Config))
func New(conf config.Config) container.DB {
	return newClient(conf)
}

// newClient builds the concrete datasource. New wraps it as a container.DB for
// callers; tests use it directly to drive the lifecycle and inspect internals.
func newClient(conf config.Config) *client {
	return &client{conf: conf}
}

// UseLogger sets the logger, satisfying GoFr's datasource instrumentation hook.
func (c *client) UseLogger(l any) {
	if logger, ok := l.(datasource.Logger); ok {
		c.logger = logger
	}
}

// UseMetrics sets the metrics recorder, satisfying GoFr's instrumentation hook.
func (c *client) UseMetrics(m any) {
	if metrics, ok := m.(gofrSQL.Metrics); ok {
		c.metrics = metrics
	}
}

// Connect establishes the connection. When IAM auth is not requested it delegates
// entirely to GoFr's standard SQL datasource (username/password over host:port).
// When DB_IAM_AUTH=true it connects through the Cloud SQL connector with IAM auth
// and wraps the result in that same standard datasource, so no SQL behavior is
// duplicated. Failures are logged and leave the datasource disconnected rather
// than panicking, matching the core SQL datasource.
func (c *client) Connect() {
	if !iamRequested(c.conf) {
		c.logger.Debugf("cloudsql: DB_IAM_AUTH not enabled; using standard SQL connection")

		// Reuse the core SQL datasource verbatim — this IS the existing connection
		// path, not a copy of it.
		c.DB = gofrSQL.NewSQL(c.conf, c.logger, c.metrics)

		return
	}

	s := parseSettings(c.conf)
	c.connectIAM(&s)
}

// connectIAM connects through the Cloud SQL connector using IAM authentication and
// wraps the opened connection in GoFr's standard SQL datasource.
func (c *client) connectIAM(s *settings) {
	if s.dialect == "" {
		c.logger.Errorf("cloudsql: unsupported dialect %q; supported are postgres and mysql", c.conf.Get("DB_DIALECT"))
		return
	}

	if s.instanceConnectionName == "" {
		c.logger.Errorf("cloudsql: DB_HOST (instance connection name 'project:region:instance') is required for IAM auth")
		return
	}

	c.logger.Debugf("cloudsql: connecting to %s instance %q via IAM auth (ipType=%s)",
		s.dialect, s.instanceConnectionName, s.ipType)

	driverName := fmt.Sprintf("cloudsql-%s-%d", s.dialect, driverSeq.Add(1))

	dsn, err := c.register(s, driverName)
	if err != nil {
		c.logger.Errorf("cloudsql: failed to register %s connector: %v", s.dialect, err)
		return
	}

	openName := driverName

	// Wrap the connector driver with otelsql so queries are traced, matching the
	// core SQL datasource. Tracing is best-effort: on failure we fall back to the
	// untraced driver rather than failing the connection.
	if traced, terr := otelsql.Register(driverName); terr == nil {
		openName = traced
	} else {
		c.logger.Warnf("cloudsql: tracing disabled, could not register otel driver: %v", terr)
	}

	db, err := sql.Open(openName, dsn)
	if err != nil {
		c.logger.Errorf("cloudsql: failed to open connection: %v", err)
		_ = c.runCleanup()

		return
	}

	c.DB = gofrSQL.NewSQLFromDB(db, s.dbConfig(), c.logger, c.metrics)
}

// register registers the Cloud SQL connector driver for the dialect under
// driverName, stores the connector cleanup function on the client and returns the
// DSN to open the connection with.
func (c *client) register(s *settings, driverName string) (string, error) {
	switch s.dialect {
	case dialectPostgres:
		cleanup, err := pgxv5.RegisterDriver(driverName, s.connectorOptions()...)
		if err != nil {
			return "", err
		}

		c.cleanup = cleanup

		return s.postgresDSN(), nil
	case dialectMySQL:
		cleanup, err := cloudmysql.RegisterDriver(driverName, s.connectorOptions()...)
		if err != nil {
			return "", err
		}

		c.cleanup = cleanup

		return s.mysqlDSN(driverName), nil
	default:
		return "", errUnsupportedDialect
	}
}

// Close closes the underlying connection and, on the IAM path, tears down the
// connector registration (which stops its background credential refresh).
func (c *client) Close() error {
	var err error

	if c.DB != nil {
		err = c.DB.Close()
	}

	return errors.Join(err, c.runCleanup())
}

func (c *client) runCleanup() error {
	if c.cleanup == nil {
		return nil
	}

	cleanup := c.cleanup
	c.cleanup = nil

	return cleanup()
}
