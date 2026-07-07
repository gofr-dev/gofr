package sql

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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
