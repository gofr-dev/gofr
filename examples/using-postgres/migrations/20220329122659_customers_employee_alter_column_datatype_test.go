package migrations

import (
	"errors"
	"io"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

//nolint:dupl // Cannot use same testCase for migrateUP and migrateDOWN
func TestK20220329122659_Up(t *testing.T) {
	mock, db := initializeTests(t)
	k := K20220329122659{}

	// register mock calls for success case
	mock.ExpectExec(AlterType).WillReturnResult(sqlmock.NewResult(1, 0))
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

//nolint:dupl // Cannot use same testCase for migrateUP and migrateDOWN
func TestK20220329122659_Down(t *testing.T) {
	mock, db := initializeTests(t)
	k := K20220329122659{}

	// register mock calls for success case
	mock.ExpectExec(ResetType).WillReturnResult(sqlmock.NewResult(1, 0))
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
