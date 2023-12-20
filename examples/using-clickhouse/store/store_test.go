package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	clickhousemock "github.com/srikanthccv/ClickHouse-go-mock"

	"gofr.dev/examples/using-clickhouse/models"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func TestUser_Create(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	ctx.Context = context.Background()

	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")

	store := New()

	tests := []struct {
		desc    string
		id      string
		user    models.User
		resp    models.User
		mockErr error
		err     error
	}{
		{"Valid case", "1", models.User{Name: "stella", Age: "21"}, models.User{ID: uid, Name: "stella", Age: "21"}, nil, nil},
		{"db error", "2", models.User{Name: "sony", Age: "21"}, models.User{ID: uid, Name: "sony", Age: "21"},
			errors.DB{Err: errors.Error("database error")}, errors.DB{Err: errors.Error("database error")}},
		{desc: "scan error", id: "5", user: models.User{Name: "stella", Age: "21"}, err: errors.DB{Err: errors.Error("scan error")}},
	}

	for i, tc := range tests {
		ctx.DataStore = datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: tc.id}}}

		_, err := store.Create(ctx, tc.user)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestUser_Get(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	ctx.Context = context.Background()

	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	uid1 := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca4")

	store := New()

	tests := []struct {
		desc string
		id   string
		resp []models.User
		err  error
	}{
		{desc: "Valid case", id: "1", resp: []models.User{{ID: uid, Name: "stella", Age: "21"}, {ID: uid1, Name: "sam", Age: "31"}}},
		{desc: "db error", id: "2", err: errors.DB{Err: errors.Error("db error")}},
		{desc: "scan error", id: "3", err: errors.DB{Err: errors.Error("scan error")}},
	}

	for i, tc := range tests {
		ctx.DataStore = datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: tc.id}}}

		resp, err := store.Get(ctx)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestUser_GetByID(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	ctx.Context = context.Background()

	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	uid1 := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca4")

	store := New()

	tests := []struct {
		desc string
		id   string
		uid  uuid.UUID
		resp models.User
		err  error
	}{
		{"Valid case", "1", uid, models.User{ID: uid, Name: "stella", Age: "21"}, nil},
		{"db error", "4", uid1, models.User{}, errors.Error("db error")},
	}

	for i, tc := range tests {
		ctx.DataStore = datastore.DataStore{ClickHouse: datastore.ClickHouseDB{Conn: mockClickhouse{id: tc.id}}}

		resp, err := store.GetByID(ctx, tc.uid)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
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
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	uid1 := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca4")

	if m.id == "1" {
		columns := []clickhousemock.ColumnType{{Name: "id", Type: "UUID"}, {Name: "name", Type: "String"}, {Name: "age", Type: "String"}}

		values := [][]interface{}{{uid, "stella", "21"}, {uid1, "sam", "31"}}

		rows := clickhousemock.NewRows(columns, values)

		return rows, nil
	} else if m.id == "2" {
		return nil, errors.Error("db error")
	}

	columns1 := []clickhousemock.ColumnType{{Name: "id", Type: "UUID"}, {Name: "name", Type: "String"}}

	value1 := [][]interface{}{{uid, "sam"}}

	rows1 := clickhousemock.NewRows(columns1, value1)

	return rows1, nil
}

func (m mockClickhouse) QueryRow(_ context.Context, _ string, _ ...any) driver.Row {
	if m.id == "1" {
		uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
		return mockRow{colNames: []string{"id", "name", "age"}, values: []interface{}{uid, "stella", "21"}}
	}

	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")

	return mockRow{colNames: []string{"id", "name"}, values: []interface{}{uid, "sony"}}
}

func (m mockClickhouse) PrepareBatch(_ context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, nil
}

func (m mockClickhouse) Exec(_ context.Context, _ string, _ ...any) error {
	if m.id == "1" || m.id == "5" {
		return nil
	}

	return errors.Error("database error")
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

type mockRow struct {
	colNames []string
	values   []any
}

func (m mockRow) Err() error {
	return nil
}
func (m mockRow) Scan(dest ...any) error {
	if len(dest) != len(m.values) {
		return errors.Error("scan error")
	}

	for i, v := range m.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(v))
	}

	return nil
}
func (m mockRow) ScanStruct(_ any) error {
	return nil
}
