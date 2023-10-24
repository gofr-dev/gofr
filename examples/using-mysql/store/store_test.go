package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-mysql/models"
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
	_, err := app.DB().Exec("DROP TABLE IF EXISTS employees;")

	if err != nil {
		return
	}

	_, err = app.DB().Exec("CREATE TABLE IF NOT EXISTS employees " +
		"(id serial primary key, name varchar(50), email varchar(50), phone varchar(20), city varchar(50));")
	if err != nil {
		return
	}
}

func TestAddCustomer(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))

	if err != nil {
		ctx.Logger.Error("mock connection failed")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()
	tests := []struct {
		desc     string
		employee models.Employee
		mockErr  error
		err      error
	}{
		{"Valid case", models.Employee{ID: 2, Name: "Test123", Email: "test@gmail.com",
			Phone: 1234567890, City: "kol"}, nil, nil},
		{"DB error", models.Employee{ID: 6, Name: "Test234", Email: "test1@gmail.com",
			Phone: 1224567890}, errors.DB{}, errors.DB{Err: errors.DB{}}},
	}

	for i, tc := range tests {
		// Set up the expectations for the INSERT query
		mock.ExpectExec("INSERT INTO employees (id, name, email, phone, city) VALUES (?, ?, ?, ?, ?)").
			WithArgs(tc.employee.ID, tc.employee.Name, tc.employee.Email, tc.employee.Phone, tc.employee.City).
			WillReturnResult(sqlmock.NewResult(2, 1)).
			WillReturnError(tc.mockErr)

		// Set up the expectations for the SELECT query
		rows := sqlmock.NewRows([]string{"id", "name", "email", "phone", "city"}).
			AddRow(tc.employee.ID, tc.employee.Name, tc.employee.Email, tc.employee.Phone, tc.employee.City)
		mock.ExpectQuery("SELECT id, name, email, phone, city FROM employees WHERE id = ?").
			WithArgs(tc.employee.ID).
			WillReturnRows(rows).
			WillReturnError(tc.mockErr)

		store := New()
		resp, err := store.Create(ctx, tc.employee)

		ctx.Logger.Log(resp)
		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestGetEmployees(t *testing.T) {
	ctx := gofr.NewContext(nil, nil, gofr.New())
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))

	if err != nil {
		ctx.Logger.Error("mock connection failed")
	}

	ctx.DataStore = datastore.DataStore{ORM: db}
	ctx.Context = context.Background()

	tests := []struct {
		desc      string
		employees []models.Employee
		mockErr   error
		err       error
	}{
		{"Valid case with employees", []models.Employee{
			{ID: 1, Name: "John Doe", Email: "john@example.com", Phone: 1234567890, City: "New York"},
			{ID: 2, Name: "Jane Smith", Email: "jane@example.com", Phone: 9876543210, City: "San Francisco"},
		}, nil, nil},
		{"Valid case with no employees", []models.Employee{}, nil, nil},
		{"Error case", nil, errors.Error("database error"), errors.DB{Err: errors.Error("database error")}},
	}

	for i, tc := range tests {
		rows := sqlmock.NewRows([]string{"id", "name", "email", "phone", "city"})
		for _, emp := range tc.employees {
			rows.AddRow(emp.ID, emp.Name, emp.Email, emp.Phone, emp.City)
		}

		mock.ExpectQuery("SELECT id,name,email,phone,city FROM employees").WillReturnRows(rows).WillReturnError(tc.mockErr)

		store := New()
		resp, err := store.Get(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.employees, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
