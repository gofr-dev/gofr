package migrations

import (
	"errors"
	"io"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

const invalidQuery = `invalid query`

func initializeTests(t *testing.T) (sqlmock.Sqlmock, datastore.DataStore) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("error %s was not expected when opening a mock database connection", err)
	}

	dataStore := datastore.DataStore{ORM: db}

	return mock, dataStore
}

//nolint:dupl //Cannot use same testCase for migrateUP and migrateDOWN
func TestK20220329122401_Up(t *testing.T) {
	mock, db := initializeTests(t)
	k := K20220329122401{}

	// register mock calls for success case
	mock.ExpectExec(CreateTable).WillReturnResult(sqlmock.NewResult(1, 0))
	// register mock calls for failure case
	mock.ExpectExec(invalidQuery).WillReturnError(errors.New("invalid migration"))

	testCases := []struct {
		desc string
		err  error
	}{
		{"success", nil},
		{"failure", errors.New("invalid migration")},
	}

	for i, tc := range testCases {
		err := k.Up(&db, log.NewMockLogger(io.Discard))

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

//nolint:dupl //Cannot use same testCase for migrateUP and migrateDOWN
func TestK20220329122401_Down(t *testing.T) {
	mock, db := initializeTests(t)
	k := K20220329122401{}

	// register mock calls for success case
	mock.ExpectExec(DroopTable).WillReturnResult(sqlmock.NewResult(1, 0))
	// register mock calls for failure case
	mock.ExpectExec(invalidQuery).WillReturnError(errors.New("invalid migration"))

	testCases := []struct {
		desc string
		err  error
	}{
		{"success", nil},
		{"failure", errors.New("invalid migration")},
	}

	for i, tc := range testCases {
		err := k.Down(&db, log.NewMockLogger(io.Discard))

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
