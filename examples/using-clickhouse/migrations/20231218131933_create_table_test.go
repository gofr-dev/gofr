package migrations

import (
	"context"
	"io"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

//nolint:dupl //Cannot use same testCase for migrateUP and migrateDOWN
func TestK20231218131933_Up(t *testing.T) {
	k := K20231218131933{}

	testCases := []struct {
		desc string
		mock datastore.DataStore
		err  error
	}{
		{"success", datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: "1"}}}, nil},
		{"failure", datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: "2"}}}, errors.New("invalid migration")},
	}

	for i, tc := range testCases {
		tc := tc

		err := k.Up(&tc.mock, log.NewMockLogger(io.Discard))

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

//nolint:dupl //Cannot use same testCase for migrateUP and migrateDOWN
func TestK20231218131933_Down(t *testing.T) {
	k := K20231218131933{}

	testCases := []struct {
		desc string
		mock datastore.DataStore
		err  error
	}{
		{"success", datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: "1"}}}, nil},
		{"failure", datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: "2"}}}, errors.New("invalid migration")},
	}

	for i, tc := range testCases {
		tc := tc

		err := k.Down(&tc.mock, log.NewMockLogger(io.Discard))

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

type mockClickhouse struct {
	id string
}

func (m mockClickhouse) Contributors() []string {
	return nil
}

func (m mockClickhouse) ServerVersion() (*driver.ServerVersion, error) {
	return nil, nil
}

func (m mockClickhouse) Select(_ context.Context, _ any, _ string, _ ...any) error {
	return nil
}

func (m mockClickhouse) Query(_ context.Context, _ string, _ ...any) (driver.Rows, error) {
	return nil, nil
}

func (m mockClickhouse) QueryRow(_ context.Context, _ string, _ ...any) driver.Row {
	return nil
}

func (m mockClickhouse) PrepareBatch(_ context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, nil
}

func (m mockClickhouse) Exec(_ context.Context, _ string, _ ...any) error {
	if m.id == "1" {
		return nil
	}

	return errors.New("database error")
}

func (m mockClickhouse) AsyncInsert(_ context.Context, _ string, _ bool, _ ...any) error {
	return nil
}

func (m mockClickhouse) Ping(context.Context) error {
	return nil
}

func (m mockClickhouse) Stats() driver.Stats {
	return driver.Stats{}
}

func (m mockClickhouse) Close() error {
	return nil
}
