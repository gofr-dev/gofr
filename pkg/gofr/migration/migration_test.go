package migration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_MigrationMySQLSuccess(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectExec("DELETE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				var (
					e int
				)

				_, err := d.DB.Exec("CREATE table customer(id int not null);")
				if err != nil {
					return err
				}

				_, err = d.DB.Query("SELECT id from customers")
				if err != nil {
					return err
				}

				err = d.DB.QueryRow("SELECT id from customers WHERE id = ?", 1).Scan(&e)
				if err != nil {
					return err
				}

				err = d.DB.QueryRowContext(context.Background(), "SELECT * FROM customers").Scan(&e)
				if err != nil {
					return err
				}

				_, err = d.DB.ExecContext(context.Background(), "DELETE FROM customers WHERE id = 1")
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
		cntnr := container.NewContainer(&config.EnvFile{})
		sqlClient, mock, _ := sqlmock.New()
		redisClient, redisMock := redismock.NewClientMock()

		cntnr.DB.DB = sqlClient

		cntnr.Redis.Client = redisClient

		start := time.Now()

		data, _ := json.Marshal(migration{
			Method:    "UP",
			StartTime: start,
			Duration:  time.Since(start).Milliseconds(),
		})

		redisMock.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{"1": string(data)})

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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

func Test_MigrationRedisGoFrDataUnmarshalFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "localhost")

	logs := testutil.StdoutOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		redisClient, redisMock := redismock.NewClientMock()

		cntnr.Redis.Client = redisClient

		start := time.Now()

		data, _ := json.Marshal(migration{
			Method:    "UP",
			StartTime: start,
			Duration:  time.Since(start).Milliseconds(),
		})

		redisMock.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{"1": string(data)[10:]})

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT.*").WillReturnError(errors.New("failed"))
		mock.ExpectRollback()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT.*").WillReturnError(errors.New("failed"))
		mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT.*").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit().WillReturnError(errors.New("failed"))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"row"}).AddRow(1))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("CREATE table customer(id int not null);")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectExec("SELECT.*").WillReturnError(errors.New("transaction failed"))
		mock.ExpectRollback()

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("SELECT 2+2")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnError(errors.New("row not found"))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("SELECT 2+2")
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
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin().WillReturnError(errors.New("failed to start transaction"))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("SELECT 2+2")
				if err != nil {
					return err
				}

				return nil
			}},
		}, cntnr)
	})

	assert.Contains(t, logs, "unable to begin transaction: failed to start transaction")
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

func Test_MigrationMySQLCreateGoFrMigrationError(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DIALECT", "mysql")

	logs := testutil.StderrOutputForFunc(func() {
		cntnr := container.NewContainer(&config.EnvFile{})
		db, mock, _ := sqlmock.New()

		cntnr.DB.DB = db

		mock.ExpectQuery("SELECT.*").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(0))
		mock.ExpectExec("CREATE.*").WillReturnError(errors.New("creation failed"))

		Run(map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				_, err := d.DB.Exec("SELECT 2+2")
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
		cntnr := container.NewContainer(&config.EnvFile{})

		client, mock := redismock.NewClientMock()

		cntnr.Redis.Client = client

		mock.ExpectHGetAll("gofr_migrations").SetVal(map[string]string{})
		mock.ExpectTxPipeline()
		mock.ExpectSet("key", "value", 0).RedisNil()
		mock.ExpectRename("key", "newKey").RedisNil()
		mock.ExpectGet("newKey")
		mock.ExpectDel("newKey")
		mock.ExpectHSet("gofr_migrations", "*")
		mock.ExpectTxPipelineExec().RedisNil()

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
		cntnr := container.NewContainer(&config.EnvFile{})

		client, mock := redismock.NewClientMock()

		cntnr.Redis.Client = client

		err := errors.New("unable to get gofr_migrations")

		mock.ExpectHGetAll("gofr_migrations").SetErr(err)

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
