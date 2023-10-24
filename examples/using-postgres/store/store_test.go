package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-postgres/model"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func TestCoreLayer(*testing.T) {
	app := gofr.New()

	// initializing the seeder
	seeder := datastore.NewSeeder(&app.DataStore, "../db")
	seeder.ResetCounter = true

	createTable(app)
}

func createTable(app *gofr.Gofr) {
	// drop table to clean previously added id's
	_, err := app.DB().Exec("DROP TABLE IF EXISTS customers;")

	if err != nil {
		return
	}

	_, err = app.DB().Exec("CREATE TABLE IF NOT EXISTS customers " +
		"(id varchar(36) PRIMARY KEY , name varchar(50) , email varchar(50) , phone bigint);")
	if err != nil {
		return
	}
}

func TestAddCustomer(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock connection failed")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()
	tests := []struct {
		desc     string
		customer model.Customer
		mockErr  error
		err      error
	}{
		{"Valid case", model.Customer{Name: "Test123", Email: "test@gmail.com", Phone: 1234567890}, nil, nil},
		{"DB error", model.Customer{Name: "Test234", Email: "test1@gmail.com", Phone: 1224567890}, errors.DB{}, errors.DB{Err: errors.DB{}}},
	}

	for i, tc := range tests {
		row := mock.NewRows([]string{"name", "email", "phone"}).AddRow(tc.customer.Name, tc.customer.Email, tc.customer.Phone)
		mock.ExpectQuery("INSERT INTO").
			WithArgs(sqlmock.AnyArg(), tc.customer.Name, tc.customer.Email, tc.customer.Phone).
			WillReturnRows(row).WillReturnError(tc.mockErr)

		store := New()
		resp, err := store.Create(ctx, tc.customer)

		ctx.Logger.Log(resp)
		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestGetCustomerByID(t *testing.T) {
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	invalidUID := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca5")
	tests := []struct {
		desc     string
		customer model.Customer
		id       uuid.UUID
		mockErr  error
		err      error
	}{
		{"Get existent id", model.Customer{ID: uid, Name: "zopsmart", Email: "Zopsmart@gmail.com", Phone: 1234567789},
			uid, nil, nil},
		{"Get non existent id", model.Customer{}, invalidUID, sql.ErrNoRows,
			errors.EntityNotFound{Entity: "customer", ID: "37387615-aead-4b28-9adc-78c1eb714ca5"}},
	}
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock is not initialized")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()

	for i, tc := range tests {
		rows := mock.NewRows([]string{"id", "name", "email", "phone"}).
			AddRow(tc.customer.ID, tc.customer.Name, tc.customer.Email, tc.customer.Phone)
		mock.ExpectQuery("SELECT id,name").WithArgs(tc.id).WillReturnRows(rows).WillReturnError(tc.mockErr)

		store := New()

		_, err := store.GetByID(ctx, tc.id)
		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestUpdateCustomer(t *testing.T) {
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	uid1 := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca1")
	tests := []struct {
		desc     string
		customer model.Customer
		err      error
	}{
		{"update success", model.Customer{ID: uid, Name: "Test1234"}, nil},
		{"update fail", model.Customer{ID: uid1, Name: "very-long-mock-name-lasdjflsdjfljasdlfjsdlfjsdfljlkj"}, errors.DB{}},
	}
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock is not initialized")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()

	for i, tc := range tests {
		mock.ExpectExec("UPDATE customers").
			WithArgs(tc.customer.Name, tc.customer.Email, tc.customer.Phone, tc.customer.ID).
			WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(tc.err)

		ctx := gofr.NewContext(nil, nil, app)
		ctx.Context = context.Background()

		store := New()

		_, err := store.Update(ctx, tc.customer)

		if _, ok := err.(errors.DB); err != nil && ok == false {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.err, err, tc.desc)
		}
	}
}

func TestGetCustomers(t *testing.T) {
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")

	tests := []struct {
		desc     string
		customer model.Customer
		mockErr  error
		err      error
	}{
		{"Get existent data", model.Customer{ID: uid, Name: "zopsmart", Email: "zopsmart@gmail.com", Phone: 123456789}, nil, nil},
		{"db connection failed", model.Customer{}, errors.DB{}, errors.DB{Err: errors.DB{}}},
	}
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock not initialized")
	}

	ctx.Context = context.Background()
	ctx.DataStore = datastore.DataStore{ORM: db}

	for i, tc := range tests {
		rows := mock.NewRows([]string{"id", "name", "email", "phone"}).
			AddRow(tc.customer.ID, tc.customer.Name, tc.customer.Email, tc.customer.Phone)
		mock.ExpectQuery("SELECT id,name").WillReturnRows(rows).WillReturnError(tc.mockErr)

		store := New()
		_, err := store.Get(ctx)
		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestGetScanErr(t *testing.T) {
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock is not initialized")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()
	rows := mock.NewRows([]string{"id", "name", "email", "phone"}).
		AddRow(1223, "Durga", "zopsmart@gmail.com", 1234567890)
	mock.ExpectQuery("SELECT id,name").WillReturnRows(rows).WillReturnError(nil)

	store := New()

	_, err = store.Get(ctx)
	if err == nil {
		t.Errorf("TEST CASE FAILED, Expected: %v, Got: %v", nil, err)
	}
}

func TestDeleteCustomer(t *testing.T) {
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")
	uid1 := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca1")
	tests := []struct {
		desc         string
		id           uuid.UUID
		mockErr      error
		rowsAffected int64
		err          error
	}{
		{"delete success test #1", uid, nil, 1, nil},
		{"delete failure test #2", uid1, errors.DB{}, 0, errors.DB{Err: errors.DB{}}},
	}
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	db, mock, err := sqlmock.New()

	if err != nil {
		ctx.Logger.Error("mock is not initialized")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()

	for i, tc := range tests {
		mock.ExpectExec("DELETE FROM customers").WithArgs(tc.id).
			WillReturnResult(sqlmock.NewResult(1, tc.rowsAffected)).
			WillReturnError(tc.mockErr)

		store := New()

		err := store.Delete(ctx, tc.id)
		if err != tc.err {
			t.Errorf("TEST CASE[%v] FAILED, Expected: %v, Got: %v", i, nil, err)
		}
	}
}
