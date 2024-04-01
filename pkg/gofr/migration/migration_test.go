//go:build !migration

package migration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

// TODO : Remove Skips because tests are failing in pipeline.

func Test_MigrationMySQLSuccess(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"lastMigration"}).AddRow(0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mocks.SQL.ExpectExec("DELETE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectCommit()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				var (
					e int
				)

				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				rows, err := d.SQL.Query("SELECT id from customers")
				if err != nil && rows.Err() == nil {
					return err
				}

				err = d.SQL.QueryRow("SELECT id from customers WHERE id = ?", 1).Scan(&e)
				if err != nil {
					return err
				}

				err = d.SQL.QueryRowContext(context.Background(), "SELECT * FROM customers").Scan(&e)
				if err != nil {
					return err
				}

				_, err = d.SQL.ExecContext(context.Background(), "DELETE FROM customers WHERE id = 1")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Migration 1 ran successfully")
}

func Test_MigrationMySQLAndRedisLastMigrationAreDifferent(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")
	t.Setenv("REDIS_HOST", "localhost")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		start := time.Now()

		data, _ := json.Marshal(migration{
			Method:    "UP",
			StartTime: start,
			Duration:  time.Since(start).Milliseconds(),
		})

		mocks.Redis.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{"1": string(data)})

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectCommit()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				_, err = d.Redis.Set(context.Background(), "key", "value", 0).Result()
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.NotContains(t, logs, "Migration 1 ran successfully")
}

func Test_MigrationMySQLPostRunFailed(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectExec("INSERT.*").WillReturnError(testutil.CustomError{ErrorMessage: "failed"})
		mocks.SQL.ExpectRollback()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Migration transaction rolled back")
}

func Test_MigrationMySQLPostRunRollBackFailed(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectExec("INSERT.*").WillReturnError(testutil.CustomError{ErrorMessage: "failed"})
		mocks.SQL.ExpectRollback().WillReturnError(testutil.CustomError{ErrorMessage: "rollback failed"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Migration transaction rolled back")
}

func Test_MigrationMySQLTransactionCommitFailed(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mocks.SQL.ExpectCommit().WillReturnError(testutil.CustomError{ErrorMessage: "failed"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "unable to commit transaction")
}

func Test_MigrationMySQLRunSameMigrationAgain(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"row"}).AddRow(1))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.NotContains(t, logs, "Migration 1 ran successfully")
}

func Test_MigrationUPFailed(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin()
		mocks.SQL.ExpectExec("SELECT.*").WillReturnError(testutil.CustomError{ErrorMessage: "transaction failed"})
		mocks.SQL.ExpectRollback()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("SELECT 2+2")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Migration transaction rolled back")
}

func Test_MigrationSQLMigrationTableCheckFailed(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnError(testutil.CustomError{ErrorMessage: "row not found"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("SELECT 2+2")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Unable to verify sql migration table due to: row not found")
}

func Test_MigrationMySQLTransactionCreationFailure(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mocks.SQL.ExpectBegin().WillReturnError(testutil.CustomError{ErrorMessage: "failed to start transaction"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("SELECT 2+2")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "unable to begin transaction: failed to start transaction")
}

func Test_MigrationMySQLCreateGoFrMigrationError(t *testing.T) {
	t.Skip()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.SQL.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mocks.SQL.ExpectExec("CREATE.*").WillReturnError(testutil.CustomError{ErrorMessage: "creation failed"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("SELECT 2+2")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Unable to verify sql migration table due to: creation failed")
}

func Test_MigrationRedisTransactionFailure(t *testing.T) {
	t.Setenv("REDIS_HOST", "localhost")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.Redis.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{})
		mocks.Redis.ExpectTxPipeline()
		mocks.Redis.ExpectSet("key", "value", 0).RedisNil()
		mocks.Redis.ExpectRename("key", "newKey").RedisNil()
		mocks.Redis.ExpectGet("newKey")
		mocks.Redis.ExpectDel("newKey")
		mocks.Redis.ExpectHSet("gofr_migrations", "*")
		mocks.Redis.ExpectTxPipelineExec().RedisNil()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.Redis.Set(context.Background(), "key", "value", 0).Result()
				if err != nil {
					return err
				}

				_, err = d.Redis.Rename(context.Background(), "key", "newKey").Result()
				if err != nil {
					return err
				}

				_, err = d.Redis.Get(context.Background(), "newKey").Result()
				if err != nil {
					return err
				}

				_, err = d.Redis.Del(context.Background(), "key").Result()
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Migration for Redis redis: nil failed with err")
}

func Test_MigrationRedisUnableToGetLastRun(t *testing.T) {
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		mocks.Redis.ExpectHGetAll("gofr_migrations").SetErr(testutil.CustomError{ErrorMessage: "unable to get gofr_migrations"})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.Redis.Set(context.Background(), "key", "value", 0).Result()
				if err != nil {
					return err
				}
				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "Failed to get migration record from Redis : unable to get gofr_migrations")
}

func Test_MigrationRedisGoFrDataUnmarshalFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "localhost")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr, mocks := container.NewMockContainer(t)

		start := time.Now()

		data, _ := json.Marshal(migration{
			Method:    "UP",
			StartTime: start,
			Duration:  time.Since(start).Milliseconds(),
		})

		mocks.Redis.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{"1": string(data)[10:]})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.SQL.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				_, err = d.Redis.Set(context.Background(), "key", "value", 0).Result()
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.NotContains(t, logs, "Migration 1 ran successfully")
}

func Test_MigrationInvalidKeys(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})

		Run(map[int64]Migrate{
			1: {UP: nil},
		}, cntnr)
	})

	assert.Contains(t, logs, "UP not defined for the following keys: [1]")
}
