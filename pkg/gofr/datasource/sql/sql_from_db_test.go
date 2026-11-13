package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

func TestNewSQLFromDB(t *testing.T) {
	openDB := func(t *testing.T) *sql.DB {
		t.Helper()

		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)

		return db
	}

	tests := []struct {
		name        string
		db          func(t *testing.T) *sql.DB
		config      *DBConfig
		wantNil     bool
		wantDialect string
	}{
		{
			name:    "nil db returns nil",
			db:      func(*testing.T) *sql.DB { return nil },
			config:  &DBConfig{Dialect: dialectPostgres},
			wantNil: true,
		},
		{
			name:    "nil config returns nil",
			db:      openDB,
			config:  nil,
			wantNil: true,
		},
		{
			name:        "wraps an open *sql.DB and wires dialect/pool",
			db:          openDB,
			config:      &DBConfig{Dialect: dialectPostgres, HostName: "10.0.0.1", Port: "5432", Database: "app"},
			wantNil:     false,
			wantDialect: dialectPostgres,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rawDB := tc.db(t)

			ctrl := gomock.NewController(t)
			metrics := NewMockMetrics(ctrl)
			metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			metrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

			got := NewSQLFromDB(rawDB, tc.config, logging.NewMockLogger(logging.DEBUG), metrics)

			if tc.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			t.Cleanup(func() { _ = got.Close() })

			// NewSQLFromDB must wrap the caller's connection (not reopen one) and
			// surface the configured dialect — that is the whole pluggable contract.
			assert.Same(t, rawDB, got.DB, "must wrap the provided *sql.DB")
			assert.Equal(t, tc.wantDialect, got.Dialect())
			assert.NotNil(t, got.HealthCheck(), "wrapped DB exposes health checks")
		})
	}
}

// okConnector is a driver.Connector that opens a fresh, always-healthy connection on
// every Connect, so a test can watch database/sql close and replace pooled connections.
type okConnector struct{}

func (okConnector) Connect(context.Context) (driver.Conn, error) { return okConn{}, nil }
func (okConnector) Driver() driver.Driver                        { return fakeDriver{} }

type okConn struct{}

func (okConn) Prepare(string) (driver.Stmt, error) { return nil, errFakeNoConn }
func (okConn) Close() error                        { return nil }
func (okConn) Begin() (driver.Tx, error)           { return nil, errFakeNoConn }

func TestNewSQLFromDB_AppliesPoolTimeLimits(t *testing.T) {
	const limit = 20 * time.Millisecond

	tests := []struct {
		name   string
		config *DBConfig
		closed func(sql.DBStats) int64
	}{
		{
			name:   "ConnMaxLifetime closes an old connection",
			config: &DBConfig{Dialect: "mysql", MaxIdleConn: 2, ConnMaxLifetime: limit},
			closed: func(s sql.DBStats) int64 { return s.MaxLifetimeClosed },
		},
		{
			name:   "ConnMaxIdleTime closes an idle connection",
			config: &DBConfig{Dialect: "mysql", MaxIdleConn: 2, ConnMaxIdleTime: limit},
			closed: func(s sql.DBStats) int64 { return s.MaxIdleTimeClosed },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewSQLFromDB(sql.OpenDB(okConnector{}), tc.config, logging.NewMockLogger(logging.ERROR), newConnectorMetrics(t))
			require.NotNil(t, got)
			t.Cleanup(func() { _ = got.Close() })

			// The connection opened by the connectivity check goes idle in the pool; database/sql's
			// background cleaner closes it once it outlives the limit, which can take up to a second.
			require.Eventually(t, func() bool {
				return tc.closed(got.DB.Stats()) > 0
			}, 5*time.Second, 2*limit)
		})
	}
}

func TestNewSQLFromDB_UnsetPoolTimeLimitsKeepCallerSettings(t *testing.T) {
	rawDB := sql.OpenDB(okConnector{})

	// Limits the caller set on its own *sql.DB must survive when the config leaves them unset.
	rawDB.SetConnMaxLifetime(20 * time.Millisecond)

	got := NewSQLFromDB(rawDB, &DBConfig{Dialect: "mysql", MaxIdleConn: 2}, logging.NewMockLogger(logging.ERROR), newConnectorMetrics(t))
	require.NotNil(t, got)
	t.Cleanup(func() { _ = got.Close() })

	require.Eventually(t, func() bool {
		return got.DB.Stats().MaxLifetimeClosed > 0
	}, 5*time.Second, 50*time.Millisecond)
}

func TestNewSQLFromConnector_ReadsPoolTimeLimits(t *testing.T) {
	conf := config.NewMockConfig(map[string]string{
		"DB_CONN_MAX_LIFETIME":  "30m",
		"DB_CONN_MAX_IDLE_TIME": "5m",
	})

	got := NewSQLFromConnector(okConnector{}, conf, logging.NewMockLogger(logging.ERROR), newConnectorMetrics(t), nil)
	require.NotNil(t, got)
	t.Cleanup(func() { _ = got.Close() })

	assert.Equal(t, 30*time.Minute, got.config.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, got.config.ConnMaxIdleTime)
}
