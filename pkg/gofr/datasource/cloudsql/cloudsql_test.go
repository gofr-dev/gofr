package cloudsql

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

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
		{"false", "false", false},
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

// TestClient_Connect_StandardSQL verifies the unified behavior: with IAM auth off,
// Connect delegates to GoFr's standard SQL datasource (here SQLite) rather than the
// Cloud SQL connector — the same Client/AddSQLDB usage works without IAM.
func TestClient_Connect_StandardSQL(t *testing.T) {
	conf := config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "sqlite",
		"DB_NAME":     filepath.Join(t.TempDir(), "test"),
		"DB_IAM_AUTH": "false",
	})

	c := New(conf)
	c.UseLogger(logging.NewMockLogger(logging.DEBUG))
	c.UseMetrics(noopMetrics{})

	c.Connect()
	t.Cleanup(func() { _ = c.Close() })

	require.NotNil(t, c.DB, "non-IAM Connect must produce a standard SQL connection")
	assert.Equal(t, "sqlite", c.DB.Dialect())
	assert.Nil(t, c.cleanup, "standard path must not register a connector")
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
			c := New(config.NewMockConfig(tc.configs))
			c.UseLogger(logging.NewMockLogger(logging.DEBUG))
			c.UseMetrics(noopMetrics{})

			c.Connect()

			assert.Nil(t, c.DB)
			assert.Nil(t, c.cleanup)
		})
	}
}
