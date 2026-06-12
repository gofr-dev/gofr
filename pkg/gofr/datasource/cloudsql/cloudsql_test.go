package cloudsql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
)

// fakeConnector is a database/sql connector that opens a real *sql.DB handle whose
// connections always fail. It lets tests drive the IAM-path wrapping (NewSQLFromDB),
// Close/cleanup and HealthCheck without a live Cloud SQL instance — the parts the
// connector itself can't be exercised without GCP.
var (
	errFakeNoConn  = errors.New("fake: connection not available in tests")
	errFakeCleanup = errors.New("connector cleanup failed")
)

type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return nil, errFakeNoConn }
func (fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errFakeNoConn }

// iamWrappedClient builds a client whose embedded DB wraps a fake connection,
// mirroring what connectIAM produces on success without needing a real connector.
func iamWrappedClient(t *testing.T) *client {
	t.Helper()

	c := newClient(config.NewMockConfig(nil))
	c.UseLogger(logging.NewMockLogger(logging.DEBUG))
	c.UseMetrics(noopMetrics{})
	c.DB = gofrSQL.NewSQLFromDB(sql.OpenDB(fakeConnector{}),
		&gofrSQL.DBConfig{Dialect: dialectPostgres, HostName: "proj:reg:inst", Database: "app"},
		c.logger, c.metrics)

	return c
}

// noopMetrics satisfies gofrSQL.Metrics for tests that exercise a real connection.
type noopMetrics struct{}

func (noopMetrics) RecordHistogram(_ context.Context, _ string, _ float64, _ ...string) {}
func (noopMetrics) SetGauge(_ string, _ float64, _ ...string)                           {}

func TestNormalizeDialect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"postgres", "postgres", dialectPostgres},
		{"postgresql alias", "postgresql", dialectPostgres},
		{"pgx alias", "pgx", dialectPostgres},
		{"mixed case + spaces", "  PostGres  ", dialectPostgres},
		{"mysql", "mysql", dialectMySQL},
		{"mysql upper", "MYSQL", dialectMySQL},
		{"unsupported", "sqlite", ""},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeDialect(tc.input))
		})
	}
}

func TestNormalizeIPType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"private", "PRIVATE", ipTypePrivate},
		{"private lower", "private", ipTypePrivate},
		{"psc", "PSC", ipTypePSC},
		{"public", "PUBLIC", ipTypePublic},
		{"empty defaults to public", "", ipTypePublic},
		{"unknown defaults to public", "internal", ipTypePublic},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeIPType(tc.input))
		})
	}
}

func TestIAMRequested(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"true mixed case", "TRUE", true},
		{"true padded", "  true  ", true},
		{"numeric one", "1", true},
		{"single char t", "t", true},
		{"false", "false", false},
		{"numeric zero", "0", false},
		{"empty", "", false},
		{"garbage", "yes", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conf := config.NewMockConfig(map[string]string{"DB_IAM_AUTH": tc.value})
			assert.Equal(t, tc.want, iamRequested(conf))
		})
	}
}

func TestSettings_DSN(t *testing.T) {
	s := settings{
		instanceConnectionName: "proj:us-central1:inst",
		user:                   "app-sa@proj.iam",
		database:               "app",
	}

	assert.Equal(t,
		"host=proj:us-central1:inst user=app-sa@proj.iam dbname=app sslmode=disable",
		s.postgresDSN())

	assert.Equal(t,
		"app-sa@proj.iam@cloudsql-mysql-1(proj:us-central1:inst)/app?parseTime=true",
		s.mysqlDSN("cloudsql-mysql-1"))
}

// TestSettings_DSN_Escaping verifies values that contain DSN-significant characters
// are quoted/escaped instead of breaking out into additional keywords.
func TestSettings_DSN_Escaping(t *testing.T) {
	// A database name carrying a space and a quote must not inject another libpq
	// keyword (e.g. sslmode=require); it stays a single quoted token.
	s := settings{
		instanceConnectionName: "proj:us-central1:inst",
		user:                   "app-sa@proj.iam",
		database:               "ap p' sslmode=require",
	}

	assert.Equal(t,
		`host=proj:us-central1:inst user=app-sa@proj.iam dbname='ap p\' sslmode=require' sslmode=disable`,
		s.postgresDSN())

	// go-sql-driver's FormatDSN owns MySQL escaping; confirm the database name is
	// not silently concatenated into the DSN unescaped.
	dsn := s.mysqlDSN("cloudsql-mysql-1")
	assert.Contains(t, dsn, "parseTime=true")
	assert.NotContains(t, dsn, "/ap p' sslmode=require?", "raw db name must not appear unescaped")
}

func TestSettings_dbConfig(t *testing.T) {
	tests := []struct {
		name        string
		settings    settings
		wantMaxIdle int
		wantMaxOpen int
	}{
		{
			name:        "defaults idle connections when unset",
			settings:    settings{instanceConnectionName: "proj:reg:inst", dialect: dialectPostgres, database: "app", user: "u"},
			wantMaxIdle: defaultMaxIdleConn,
			wantMaxOpen: 0,
		},
		{
			name:        "respects explicit pool sizing",
			settings:    settings{instanceConnectionName: "proj:reg:inst", dialect: dialectMySQL, maxIdleConn: 5, maxOpenConn: 10},
			wantMaxIdle: 5,
			wantMaxOpen: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.settings.dbConfig()

			assert.Equal(t, tc.settings.dialect, got.Dialect)
			assert.Equal(t, tc.settings.instanceConnectionName, got.HostName)
			assert.Equal(t, tc.wantMaxIdle, got.MaxIdleConn)
			assert.Equal(t, tc.wantMaxOpen, got.MaxOpenConn)
		})
	}
}

func TestSettings_connectorOptions(t *testing.T) {
	// IAM auth plus exactly one IP-type dial option, for every IP type.
	for _, ip := range []string{ipTypePublic, ipTypePrivate, ipTypePSC, ""} {
		s := settings{ipType: normalizeIPType(ip)}
		assert.Len(t, s.connectorOptions(), 2, "ip=%q", ip)
	}
}

// TestNew_ReturnsUsableDB verifies the public constructor returns a container.DB
// whose lifecycle hooks (the ones App.AddSQLDB drives) are wired, without leaking
// the concrete type.
func TestNew_ReturnsUsableDB(t *testing.T) {
	db := New(config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "sqlite",
		"DB_NAME":     filepath.Join(t.TempDir(), "test"),
		"DB_IAM_AUTH": "false",
	}))
	require.NotNil(t, db)

	// App.AddSQLDB reaches the lifecycle via duck typing; confirm New's value exposes it.
	db.(interface{ UseLogger(any) }).UseLogger(logging.NewMockLogger(logging.DEBUG))
	db.(interface{ UseMetrics(any) }).UseMetrics(noopMetrics{})
	db.(interface{ Connect() }).Connect()
	t.Cleanup(func() { _ = db.Close() })

	assert.Equal(t, "sqlite", db.Dialect())
}

// TestClient_Connect_StandardSQL verifies the unified behavior: with IAM auth off,
// Connect delegates to GoFr's standard SQL datasource (here SQLite) rather than the
// Cloud SQL connector — the same datasource/AddSQLDB usage works without IAM.
func TestClient_Connect_StandardSQL(t *testing.T) {
	conf := config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "sqlite",
		"DB_NAME":     filepath.Join(t.TempDir(), "test"),
		"DB_IAM_AUTH": "false",
	})

	c := newClient(conf)
	c.UseLogger(logging.NewMockLogger(logging.DEBUG))
	c.UseMetrics(noopMetrics{})

	c.Connect()
	t.Cleanup(func() { _ = c.Close() })

	require.NotNil(t, c.DB, "non-IAM Connect must produce a standard SQL connection")
	assert.Equal(t, "sqlite", c.DB.Dialect())
	assert.Nil(t, c.cleanup, "standard path must not register a connector")
}

// TestClient_Close_RunsCleanupAndJoinsErrors covers the IAM-path teardown that
// can't be reached without a live connector: Close closes the wrapped DB and runs
// the connector cleanup, joining both errors, and runCleanup is one-shot.
func TestClient_Close_RunsCleanupAndJoinsErrors(t *testing.T) {
	c := iamWrappedClient(t)

	calls := 0
	c.cleanup = func() error {
		calls++
		return errFakeCleanup
	}

	err := c.Close()

	require.ErrorIs(t, err, errFakeCleanup, "Close must surface the connector cleanup error")
	assert.Equal(t, 1, calls, "cleanup runs exactly once")
	assert.Nil(t, c.cleanup, "runCleanup clears the cleanup func so it can't run twice")

	require.NoError(t, c.runCleanup(), "runCleanup is a no-op once cleanup is cleared")
	assert.Equal(t, 1, calls, "cleared cleanup is not invoked again")
}

// TestClient_Close_NilCleanup verifies Close is safe on the standard path, where no
// connector was registered (cleanup is nil).
func TestClient_Close_NilCleanup(t *testing.T) {
	c := iamWrappedClient(t)
	c.cleanup = nil

	assert.NoError(t, c.Close())
}

// TestClient_HealthCheck verifies the nil-DB guard added for the failed-connect
// case: a client installed as container.SQL with a nil embedded DB must report down
// instead of panicking in the promoted HealthCheck, and otherwise delegate.
func TestClient_HealthCheck(t *testing.T) {
	t.Run("nil DB reports down without panicking", func(t *testing.T) {
		c := newClient(config.NewMockConfig(nil))

		h := c.HealthCheck()

		require.NotNil(t, h)
		assert.Equal(t, datasource.StatusDown, h.Status)
	})

	t.Run("delegates to the wrapped DB when connected", func(t *testing.T) {
		c := iamWrappedClient(t)
		t.Cleanup(func() { _ = c.Close() })

		h := c.HealthCheck()

		require.NotNil(t, h)
		// The fake connection can't ping, so it's down — but it reaches the embedded
		// DB's health logic (host detail populated) rather than the nil guard above.
		assert.Equal(t, datasource.StatusDown, h.Status)
		assert.Contains(t, h.Details, "host")
	})
}

// TestClient_Connect_IAMValidation verifies the IAM path validates configuration
// and fails safe (no panic, no connection) instead of registering a connector.
func TestClient_Connect_IAMValidation(t *testing.T) {
	tests := []struct {
		name    string
		configs map[string]string
	}{
		{"unsupported dialect", map[string]string{"DB_IAM_AUTH": "true", "DB_DIALECT": "mongo", "DB_HOST": "p:r:i"}},
		{"missing instance connection name", map[string]string{"DB_IAM_AUTH": "true", "DB_DIALECT": "postgres"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(config.NewMockConfig(tc.configs))
			c.UseLogger(logging.NewMockLogger(logging.DEBUG))
			c.UseMetrics(noopMetrics{})

			c.Connect()

			assert.Nil(t, c.DB)
			assert.Nil(t, c.cleanup)
		})
	}
}
