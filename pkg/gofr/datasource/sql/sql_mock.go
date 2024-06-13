package sql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func NewSQLMocks(t *testing.T) (*DB, sqlmock.Sqlmock, *MockMetrics) {
	return NewSQLMocksWithConfig(t, nil)
}

func NewSQLMocksWithConfig(t *testing.T, config *DBConfig) (*DB, sqlmock.Sqlmock, *MockMetrics) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats", gomock.Any(),
		"hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).AnyTimes()

	return &DB{
		DB:      db,
		logger:  logging.NewMockLogger(logging.DEBUG),
		config:  config,
		metrics: mockMetrics,
	}, mock, mockMetrics
}
