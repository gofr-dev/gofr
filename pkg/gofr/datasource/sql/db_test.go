package sql

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errDB     = testutil.CustomError{ErrorMessage: "DB error"}
	errSyntax = testutil.CustomError{ErrorMessage: "syntax error"}
	errTx     = testutil.CustomError{ErrorMessage: "error starting transaction"}
	errbegin  = testutil.CustomError{ErrorMessage: "begin failed"}
)

func getDB(t *testing.T, logLevel logging.Level) (*DB, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	db := &DB{mockDB, logging.NewMockLogger(logLevel), nil, nil}
	db.config = &DBConfig{}

	return db, mock
}

func TestDB_SelectSingleColumnFromIntToString(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	ids := make([]string, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToString(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	ids := make([]string, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromIntToInt(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	ids := make([]int, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []int{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromIntToCustomInt(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type CustomInt int

	ids := make([]CustomInt, 0)

	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToCustomInt(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type CustomInt int

	ids := make([]CustomInt, 0)

	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectContextError(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Microsecond))

	time.Sleep(1 * time.Millisecond)

	defer cancel()

	db, _ := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	// the query won't run, since context is past deadline and the function will simply return
	db.Select(ctx, nil, "select 1")
}

func TestDB_SelectDataPointerError(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		db, _ := getDB(t, logging.INFO)
		defer db.DB.Close()

		db.Select(t.Context(), nil, "select 1")
	})

	assert.Contains(t, out, "we did not get a pointer. data is not settable.", "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToCustomString(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("select id from users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type CustomStr string

	ids := make([]CustomStr, 0)

	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []CustomStr{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumn(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150")
	mock.ExpectQuery("select 1 user").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string
	}

	u := user{}

	db.Select(t.Context(), &u, "select 1 user")

	assert.Equal(t, user{
		Name:  "Vikash",
		ID:    1,
		Image: "http://via.placeholder.com/150",
	}, u, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image_url"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150")
	mock.ExpectQuery("select 1 user").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	u := user{}

	db.Select(t.Context(), &u, "select 1 user")

	assert.Equal(t, user{
		Name:  "Vikash",
		ID:    1,
		Image: "http://via.placeholder.com/150",
	}, u, "TEST Failed.\n")
}

func TestDB_SelectMultiRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image_url"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150").
		AddRow("2", "Gofr", "")
	mock.ExpectQuery("select users").
		WillReturnRows(rows)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	db.metrics = mockMetrics
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	users := []user{}

	db.Select(t.Context(), &users, "select users")

	assert.Equal(t, []user{
		{
			Name:  "Vikash",
			ID:    1,
			Image: "http://via.placeholder.com/150",
		},
		{
			Name: "Gofr",
			ID:   2,
		},
	}, users, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnError(t *testing.T) {
	ids := make([]string, 0)

	out := testutil.StderrOutputForFunc(func() {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").
			WillReturnError(errDB)

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)
		db.metrics = mockMetrics
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

		db.Select(t.Context(), &ids, "select id from users")
	})

	assert.Contains(t, out, "DB error", "TEST Failed.\n")

	assert.Equal(t, []string{}, ids, "TEST Failed.\n")
}

func TestDB_SelectDataPointerNotExpected(t *testing.T) {
	m := make(map[int]int)

	out := testutil.StdoutOutputForFunc(func() {
		db, _ := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		db.Select(t.Context(), &m, "select id from users")
	})

	assert.Contains(t, out, "a pointer to map was not expected.", "TEST Failed.\n")
}

func TestDB_Query(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT 1").
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = db.Query("SELECT 1")
		require.NoError(t, err)
		require.NoError(t, rows.Err())
		assert.NotNil(t, rows)
	})

	assert.Contains(t, out, "Query SELECT 1")
}

func TestDB_QueryError(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT ").
			WillReturnError(errSyntax)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = db.Query("SELECT")
		if !assert.Nil(t, rows) {
			require.NoError(t, rows.Err())
		}

		require.Error(t, err)
		assert.Equal(t, errSyntax, err)
	})

	assert.Contains(t, out, "Query SELECT")
}

func TestDB_QueryContext(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT 1").
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = db.QueryContext(t.Context(), "SELECT 1")
		require.NoError(t, err)
		require.NoError(t, rows.Err())
		assert.NotNil(t, rows)
	})

	assert.Contains(t, out, "QueryContext SELECT 1")
}

func TestDB_QueryContextError(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT ").
			WillReturnError(errSyntax)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = db.QueryContext(t.Context(), "SELECT")
		if !assert.Nil(t, rows) {
			require.NoError(t, rows.Err())
		}

		require.Error(t, err)
		assert.Equal(t, errSyntax, err)
	})

	assert.Contains(t, out, "QueryContext SELECT")
}

func TestDB_QueryRow(t *testing.T) {
	var (
		row *sql.Row
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("jhon"))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		row = db.QueryRow("SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRow SELECT name FROM employee WHERE id = ?")
}

func TestDB_QueryRowContext(t *testing.T) {
	var (
		row *sql.Row
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		row = db.QueryRowContext(t.Context(), "SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRowContext SELECT name FROM employee WHERE id = ?")
}

func TestDB_Exec(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = db.Exec("INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "Exec INSERT INTO employee VALUES(?, ?)")
}

func TestDB_ExecError(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectExec("INSERT INTO employee VALUES(?, ?").
			WithArgs(2, "doe").WillReturnError(errSyntax)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = db.Exec("INSERT INTO employee VALUES(?, ?", 2, "doe")
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, errSyntax, err)
	})

	assert.Contains(t, out, "Exec INSERT INTO employee VALUES(?, ?")
}

func TestDB_ExecContext(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectExec(`INSERT INTO employee VALUES(?, ?)`).
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = db.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "ExecContext INSERT INTO employee VALUES(?, ?)")
}

func TestDB_ExecContextError(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectExec(`INSERT INTO employee VALUES(?, ?)`).
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = db.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "ExecContext INSERT INTO employee VALUES(?, ?)")
}

func TestDB_Prepare(t *testing.T) {
	var (
		stmt *sql.Stmt
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		stmt, err = db.Prepare("SELECT name FROM employee WHERE id = ?")
		require.NoError(t, err)
		assert.NotNil(t, stmt)
	})

	assert.Contains(t, out, "Prepare SELECT name FROM employee WHERE id = ?")
}

func TestDB_PrepareError(t *testing.T) {
	var (
		stmt *sql.Stmt
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		stmt, err = db.Prepare("SELECT name FROM employee WHERE id = ?")
		require.NoError(t, err)
		assert.NotNil(t, stmt)
	})

	assert.Contains(t, out, "Prepare SELECT name FROM employee WHERE id = ?")
}

func TestDB_Begin(t *testing.T) {
	db, mock := getDB(t, logging.INFO)

	mock.ExpectBegin()

	tx, err := db.Begin()

	assert.NotNil(t, tx)
	require.NoError(t, err)
}

func TestDB_BeginError(t *testing.T) {
	db, mock := getDB(t, logging.INFO)

	mock.ExpectBegin().WillReturnError(errTx)

	tx, err := db.Begin()

	assert.Nil(t, tx)
	require.Error(t, err)
	assert.Equal(t, errTx, err)
}

func TestDB_Close(t *testing.T) {
	db, mock := getDB(t, logging.INFO)

	mock.ExpectClose()

	err := db.Close()

	require.NoError(t, err)
}

func getTransaction(db *DB, mock sqlmock.Sqlmock) *Tx {
	mock.ExpectBegin()

	tx, _ := db.Begin()

	return tx
}

func TestTx_Query(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		defer db.DB.Close()

		mock.ExpectQuery("SELECT 1").
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = tx.Query("SELECT 1")
		require.NoError(t, err)
		assert.NotNil(t, rows)
		require.NoError(t, rows.Err())
	})

	assert.Contains(t, out, "Query SELECT 1")
}

func TestTx_QueryError(t *testing.T) {
	var (
		rows *sql.Rows
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics
		tx := getTransaction(db, mock)

		mock.ExpectQuery("SELECT ").
			WillReturnError(errSyntax)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		rows, err = tx.Query("SELECT")
		if !assert.Nil(t, rows) {
			require.NoError(t, rows.Err())
		}

		require.Error(t, err)
		assert.Equal(t, errSyntax, err)
	})

	assert.Contains(t, out, "Query SELECT")
}

func TestTx_QueryRow(t *testing.T) {
	var (
		row *sql.Row
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("jhon"))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		row = tx.QueryRow("SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRow SELECT name FROM employee WHERE id = ?")
}

func TestTx_QueryRowContext(t *testing.T) {
	var (
		row *sql.Row
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		row = tx.QueryRowContext(t.Context(), "SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRowContext SELECT name FROM employee WHERE id = ?")
}

func TestTx_Exec(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = tx.Exec("INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "TxExec INSERT INTO employee VALUES(?, ?)")
}

func TestTx_ExecError(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectExec("INSERT INTO employee VALUES(?, ?").
			WithArgs(2, "doe").WillReturnError(errSyntax)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = tx.Exec("INSERT INTO employee VALUES(?, ?", 2, "doe")
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, errSyntax, err)
	})

	assert.Contains(t, out, "TxExec INSERT INTO employee VALUES(?, ?")
}

func TestTx_ExecContext(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectExec(`INSERT INTO employee VALUES(?, ?)`).
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = tx.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "ExecContext INSERT INTO employee VALUES(?, ?)")
}

func TestTx_ExecContextError(t *testing.T) {
	var (
		res sql.Result
		err error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectExec(`INSERT INTO employee VALUES(?, ?)`).
			WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "INSERT")

		res, err = tx.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	assert.Contains(t, out, "ExecContext INSERT INTO employee VALUES(?, ?)")
}

func TestTx_Prepare(t *testing.T) {
	var (
		stmt *sql.Stmt
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		stmt, err = tx.Prepare("SELECT name FROM employee WHERE id = ?")
		require.NoError(t, err)
		assert.NotNil(t, stmt)
	})

	assert.Contains(t, out, "Prepare SELECT name FROM employee WHERE id = ?")
}

func TestTx_PrepareError(t *testing.T) {
	var (
		stmt *sql.Stmt
		err  error
	)

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		db.metrics = mockMetrics

		tx := getTransaction(db, mock)

		mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "SELECT")

		stmt, err = tx.Prepare("SELECT name FROM employee WHERE id = ?")
		require.NoError(t, err)
		assert.NotNil(t, stmt)
	})

	assert.Contains(t, out, "Prepare SELECT name FROM employee WHERE id = ?")
}

func TestTx_Commit(t *testing.T) {
	var err error

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		defer db.DB.Close()

		db.metrics = mockMetrics
		tx := getTransaction(db, mock)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "COMMIT")
		mock.ExpectCommit()

		err = tx.Commit()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "TxCommit COMMIT")
}

func TestTx_CommitError(t *testing.T) {
	var err error

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		defer db.DB.Close()

		db.metrics = mockMetrics
		tx := getTransaction(db, mock)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "COMMIT")
		mock.ExpectCommit().WillReturnError(errDB)

		err = tx.Commit()
		require.Error(t, err)
		assert.Equal(t, errDB, err)
	})

	assert.Contains(t, out, "TxCommit COMMIT")
}

func TestTx_RollBack(t *testing.T) {
	var err error

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		defer db.DB.Close()

		db.metrics = mockMetrics
		tx := getTransaction(db, mock)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "ROLLBACK")
		mock.ExpectRollback()

		err = tx.Rollback()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "TxRollback ROLLBACK")
}

func TestTx_RollbackError(t *testing.T) {
	var err error

	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		ctrl := gomock.NewController(t)
		mockMetrics := NewMockMetrics(ctrl)

		defer db.DB.Close()

		db.metrics = mockMetrics
		tx := getTransaction(db, mock)

		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
			gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", "ROLLBACK")
		mock.ExpectRollback().WillReturnError(errDB)

		err = tx.Rollback()
		require.Error(t, err)
		assert.Equal(t, errDB, err)
	})

	assert.Contains(t, out, "TxRollback ROLLBACK")
}

func TestPrettyPrint(t *testing.T) {
	b := make([]byte, 0)
	w := bytes.NewBuffer(b)
	l := &Log{
		Type:     "Query",
		Query:    "SELECT 2 + 2",
		Duration: 12912,
	}

	l.PrettyPrint(w)

	assert.Equal(t,
		"\u001B[38;5;8mQuery                            "+
			"\u001B[38;5;24mSQL   \u001B[0m    12912\u001B[38;5;8mµs\u001B[0m SELECT 2 + 2\n",
		w.String())
}

func TestClean(t *testing.T) {
	query := ""

	out := clean(query)

	assert.Empty(t, out)
}
func TestDB_CloseWhenNil(t *testing.T) {
	db := &DB{}
	err := db.Close()
	assert.NoError(t, err)
}
func TestDB_BeginTx(t *testing.T) {
	db, mock := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	ctrl := gomock.NewController(t)
	db.metrics = NewMockMetrics(ctrl)

	mock.ExpectBegin()

	tx, err := db.BeginTx(t.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	require.NoError(t, err)
	assert.NotNil(t, tx)
}
func TestDB_PingSuccess(t *testing.T) {
	db, mock := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	ctrl := gomock.NewController(t)
	db.metrics = NewMockMetrics(ctrl)

	mock.ExpectPing()
	mock.ExpectPing().WillReturnError(nil)

	err := db.PingContext(t.Context())
	assert.NoError(t, err)
}

func TestDB_PingFailure(t *testing.T) {
	db, mock := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	mock.ExpectPing().WillReturnError(sql.ErrConnDone)

	err := db.PingContext(t.Context())
	assert.Equal(t, sql.ErrConnDone, err)
}
func TestGetOperationType_EdgeCases(t *testing.T) {
	require.Empty(t, getOperationType(""))
	require.Empty(t, getOperationType("   "))
	require.Equal(t, "SELECT", getOperationType("  SELECT * FROM users"))
}
func TestClean_EmptyString(t *testing.T) {
	require.Empty(t, clean(""))
	require.Equal(t, "SELECT", clean("  SELECT  "))
}
func TestDB_Dialect(t *testing.T) {
	db, _ := getDB(t, logging.INFO)
	defer db.Close()

	db.config.Dialect = "postgresql"
	require.Equal(t, "postgresql", db.Dialect())
}
func TestGetOperationType(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"SELECT * FROM users", "SELECT"},
		{"  INSERT INTO users", "INSERT"},
		{"UPDATE users SET name = ?", "UPDATE"},
		{"DELETE FROM users", "DELETE"},
		{"", ""},
	}

	for _, test := range tests {
		result := getOperationType(test.query)
		require.Equal(t, test.expected, result)
	}
}
func TestDB_Begin_Error(t *testing.T) {
	db, mock := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	mock.ExpectBegin().WillReturnError(errbegin)

	tx, err := db.Begin()
	require.Error(t, err)
	assert.Nil(t, tx)
}

func TestDB_sendOperationStats_RecordsMilliseconds(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)

	db := &DB{
		logger:  logging.NewMockLogger(logging.DEBUG),
		config:  &DBConfig{HostName: "host", Database: "db"},
		metrics: mockMetrics,
	}

	start := time.Now().Add(-1500 * time.Millisecond) // 1.5 seconds ago

	// Expect RecordHistogram to be called with duration 1500 (milliseconds)
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), "app_sql_stats", float64(1500),
		"hostname", "host", "database", "db", "type", "SELECT",
	)

	db.sendOperationStats(start, "SELECT", "SELECT * FROM users")

	duration := time.Since(start).Milliseconds()
	assert.Equal(t, int64(1500), duration)
}
