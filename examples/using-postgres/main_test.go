package main

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/cmd/gofr/migration"
	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
	"gofr.dev/examples/using-postgres/migrations"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/log"
)

const (
	selectQuery             = "SELECT * from customers"
	selectInformationSchema = "SELECT * FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = 'customers' AND COLUMN_NAME = 'country'"
	insertQuery1            = `INSERT INTO customers VALUES ('37387615-aead-4b28-9adc-78c1eb714ca4','qwerty','yups@zopsmart.com',1234567890);`
	insertQuery2            = `INSERT INTO customers VALUES ('37387615-aead-4b28-9adc-78c1eb714ca8','steve','golang@zopsmart.com',8899667722);`
	insertQueryUpdatedID    = `INSERT INTO customers VALUES ('37387615-aead-4b28-9adc-78c1eb714ca1','yash','yash@zopsmart.com',8899667722)`
)

var version int

func TestMain(m *testing.M) {
	app := gofr.New()

	db := app.DB()
	if db == nil {
		return
	}

	query := `
 	   CREATE TABLE IF NOT EXISTS customers 
		(id varchar(36) PRIMARY KEY , name varchar(50) , 
		email varchar(50) , phone bigint);
	`

	if app.Config.Get("DB_DIALECT") == "mssql" {
		query = `
		IF NOT EXISTS
	(  SELECT [name]
		FROM sys.tables
      WHERE [name] = 'customers') CREATE TABLE IF NOT EXISTS customers 
		(id int PRIMARY KEY identity(1,1), name varchar(50) , 
		email varchar(50) , phone bigint);
	`
	}

	if _, err := db.Exec(query); err != nil {
		app.Logger.Errorf("got error sourcing the schema: ", err)
	}

	os.Exit(m.Run())
}

func TestIntegration(t *testing.T) {
	go main()
	time.Sleep(time.Second * 5)

	req, _ := request.NewMock(http.MethodGet, "http://localhost:9092/customer", nil)
	c := http.Client{}

	resp, err := c.Do(req)
	if err != nil {
		t.Errorf("TEST Failed.\tHTTP request encountered Err: %v\n", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Failed.\tExpected %v\tGot %v\n", http.StatusOK, resp.StatusCode)
	}

	_ = resp.Body.Close()
}

func initializeTests(t *testing.T) *gofr.Gofr {
	app := gofr.New()

	db := app.DB()
	if db == nil {
		t.Errorf("db is nil")

		return nil
	}

	_, err := db.Exec("DROP TABLE If EXISTS customers")
	if err != nil {
		t.Errorf("Error in dropping customers tables %v", err)
	}

	_, err = db.Exec("DROP TABLE If EXISTS gofr_migrations")
	if err != nil {
		t.Errorf("Error in dropping gofr_migrations tables %v", err)
	}

	return app
}

func cleanUpTest(t *testing.T, app *gofr.Gofr, tableName string) {
	_, err := app.DB().Exec("DROP TABLE If EXISTS " + tableName + " ,gofr_migrations")
	if err != nil {
		t.Errorf("Error in dropping the tables %v", err)
	}
}

func Test_MigrationIntegrationUnpopulatedDatabase(t *testing.T) {
	app := initializeTests(t)

	defer cleanUpTest(t, app, "customers")

	appName := app.Config.Get("APP_NAME")

	tests := []struct {
		desc      string
		migrator  dbmigration.Migrator
		timeStamp int
		query     string
		err       error
	}{
		{"Create Table Migration", migrations.K20220329122401{}, 20220329122401, selectQuery, nil},
		{"Add new column to table ", migrations.K20220329122459{}, 20220329122459, selectInformationSchema, nil},
		{"Modify column data type", migrations.K20220329122659{}, 20220329122659, insertQuery1, nil},
	}

	for i, tc := range tests {
		_ = migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
			map[string]dbmigration.Migrator{strconv.Itoa(tc.timeStamp): tc.migrator},
			dbmigration.UP, log.NewMockLogger(io.Discard))

		_, err := app.DB().Exec(tc.query)
		if err != nil {
			t.Errorf("TEST[%d],Received unexpected error:\n%+v", i, err)

			continue
		}

		err = app.DB().QueryRow("SELECT version from gofr_migrations ORDER BY end_time DESC LIMIT 1").Scan(&version)
		if err != nil {
			t.Errorf("TEST[%d],Received unexpected error:\n%+v", i, err)

			continue
		}

		assert.Equal(t, tc.timeStamp, version, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_MigrationIntegrationPopulatedDatabase(t *testing.T) {
	app := initializeTests(t)

	defer cleanUpTest(t, app, "customers")

	appName := app.Config.Get("APP_NAME")

	// creating customers table
	err := migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
		map[string]dbmigration.Migrator{strconv.Itoa(20220329122401): migrations.K20220329122401{}}, dbmigration.UP,
		log.NewMockLogger(io.Discard))
	if err != nil {
		t.Errorf("Error in migration : %v", err)
	}

	// seeding customers table with data
	seeder := datastore.NewSeeder(&app.DataStore, "./db")
	seeder.RefreshTables(t, "customers"+
		"")

	tests := []struct {
		desc      string
		migrator  dbmigration.Migrator
		timeStamp int
		query     string
	}{
		{"Add not null column with default data migration",
			migrations.K20220329123813{}, 20220329123813, insertQuery2},
		{"Change data-type of primary key", migrations.K20220329123903{},
			20220329123903, insertQueryUpdatedID},
	}

	for i, tc := range tests {
		err = migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
			map[string]dbmigration.Migrator{strconv.Itoa(tc.timeStamp): tc.migrator}, dbmigration.UP,
			log.NewMockLogger(io.Discard))

		assert.NoError(t, err, "TEST[%d]", i)

		_, err := app.DB().Exec(tc.query)

		assert.NoError(t, err, "TEST[%d]", i)

		er := app.DB().QueryRow("SELECT version from gofr_migrations ORDER BY end_time DESC LIMIT 1").Scan(&version)

		assert.NoError(t, er, "TEST[%d]", i)

		assert.Equal(t, tc.timeStamp, version, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func initializeTestsDownMethods(t *testing.T) (app *gofr.Gofr, appName string) {
	app = gofr.New()

	db := app.DB()
	if db == nil {
		t.Errorf("db is nil")

		return nil, ""
	}

	appName = app.Config.Get("APP_NAME")

	mgr := []struct {
		name      string
		migrator  dbmigration.Migrator
		timeStamp int
	}{
		{"Create table", migrations.K20220329122401{}, 20220329122401},
		{"Add column", migrations.K20220329122459{}, 20220329122459},
		{"Alter column data-type", migrations.K20220329122659{}, 20220329122659},
		{"Add not-null column", migrations.K20220329123813{}, 20220329123813},
		{"Alter primary key", migrations.K20220329123903{}, 20220329123903},
		{"Drop Column", migrations.K20230518180017{}, 20230518180017},
	}

	for i, tc := range mgr {
		err := migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
			map[string]dbmigration.Migrator{strconv.Itoa(tc.timeStamp): tc.migrator}, dbmigration.UP,
			log.NewMockLogger(io.Discard))
		if err != nil {
			t.Errorf("Error in migration %v:\n Desc: %v \n: Error : %v\n", i+1, tc.name, err)
		}
	}

	return app, appName
}

func Test_MigrationIntegrationDownMethods(t *testing.T) {
	app, appName := initializeTestsDownMethods(t)
	if appName == "" || app == nil {
		return
	}

	defer cleanUpTest(t, app, "customers")

	tests := []struct {
		desc      string
		migrator  dbmigration.Migrator
		timeStamp int
	}{
		{"Create column", migrations.K20230518180017{}, 20230518180017},
		{"Reset primary key", migrations.K20220329123903{}, 20220329123903},
		{"Drop not-null Column", migrations.K20220329123813{}, 20220329123813},
		{"Reset column data-type", migrations.K20220329122659{}, 20220329122659},
		{"Drop column", migrations.K20220329122459{}, 20220329122459},
		{"Drop tables", migrations.K20220329122401{}, 20220329122401},
	}

	for i, tc := range tests {
		err := migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
			map[string]dbmigration.Migrator{strconv.Itoa(tc.timeStamp): tc.migrator}, "DOWN",
			log.NewMockLogger(io.Discard))

		assert.NoError(t, err, "TEST[%d]", i+1)

		er := app.DB().QueryRow("SELECT version from gofr_migrations ORDER BY end_time DESC LIMIT 1").Scan(&version)

		assert.NoError(t, er, "TEST[%d]", i)

		assert.Equal(t, tc.timeStamp, version, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
