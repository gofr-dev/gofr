package sql

import (
	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
)

func NewSQLMocks(t *testing.T) (*DB, sqlmock.Sqlmock, *MockMetrics) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)

	return &DB{
		DB:      db,
		logger:  testutil.NewMockLogger(testutil.DEBUGLOG),
		config:  nil,
		metrics: mockMetrics,
	}, mock, mockMetrics
}
