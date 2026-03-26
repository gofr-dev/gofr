package sql

import (
	"bytes"
	"context"
	"database/sql"
	"sync"
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
	errBegin  = testutil.CustomError{ErrorMessage: "begin failed"}
)

func getDB(t *testing.T, logLevel logging.Level) (*DB, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	db := &DB{
		DB:         mockDB,
		logger:     logging.NewMockLogger(logLevel),
		config:     &DBConfig{},
		stopSignal: make(chan struct{}),
		closeOnce:  sync.Once{},
	}

	return db, mock
}

func getTransaction(db *DB, mock sqlmock.Sqlmock) *Tx {
	mock.ExpectBegin()

	tx, _ := db.Begin()

	return tx
}

// setupMetrics attaches a mock Metrics to db with one RecordHistogram expectation.
// opType accepts a specific string (e.g. "SELECT") or gomock.Any().
func setupMetrics(t *testing.T, db *DB, opType any) {
	t.Helper()

	m := NewMockMetrics(gomock.NewController(t))
	db.metrics = m
	m.EXPECT().RecordHistogram(gomock.Any(), "app_sql_stats",
		gomock.Any(), "hostname", gomock.Any(), "database", gomock.Any(), "type", opType)
}

// --- DB.Select tests ---

func TestDB_SelectSingleColumnFromIntToString(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select id from users").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	setupMetrics(t, db, gomock.Any())

	ids := make([]string, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToString(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select id from users").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("1").AddRow("2"))
	setupMetrics(t, db, gomock.Any())

	ids := make([]string, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromIntToInt(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select id from users").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	setupMetrics(t, db, gomock.Any())

	ids := make([]int, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.Equal(t, []int{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnCustomTypes(t *testing.T) {
	type (
		CustomInt int
		CustomStr string
	)

	t.Run("int to CustomInt", func(t *testing.T) {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
		setupMetrics(t, db, gomock.Any())

		ids := make([]CustomInt, 0)
		db.Select(t.Context(), &ids, "select id from users")

		assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
	})

	t.Run("string to CustomInt", func(t *testing.T) {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("1").AddRow("2"))
		setupMetrics(t, db, gomock.Any())

		ids := make([]CustomInt, 0)
		db.Select(t.Context(), &ids, "select id from users")

		assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
	})

	t.Run("string to CustomStr", func(t *testing.T) {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("1").AddRow("2"))
		setupMetrics(t, db, gomock.Any())

		ids := make([]CustomStr, 0)
		db.Select(t.Context(), &ids, "select id from users")

		assert.Equal(t, []CustomStr{"1", "2"}, ids, "TEST Failed.\n")
	})
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

func TestDB_SelectDataPointerNotExpected(t *testing.T) {
	m := make(map[int]int)

	out := testutil.StdoutOutputForFunc(func() {
		db, _ := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		db.Select(t.Context(), &m, "select id from users")
	})

	assert.Contains(t, out, "a pointer to map was not expected.", "TEST Failed.\n")
}

func TestDB_SelectSingleColumnError(t *testing.T) {
	ids := make([]string, 0)

	out := testutil.StderrOutputForFunc(func() {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").WillReturnError(errDB)
		setupMetrics(t, db, gomock.Any())

		db.Select(t.Context(), &ids, "select id from users")
	})

	assert.Contains(t, out, "DB error", "TEST Failed.\n")
	assert.Equal(t, []string{}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumn(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select 1 user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image"}).
			AddRow("1", "Vikash", "http://via.placeholder.com/150"))
	setupMetrics(t, db, gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string
	}

	u := user{}
	db.Select(t.Context(), &u, "select 1 user")

	assert.Equal(t, user{Name: "Vikash", ID: 1, Image: "http://via.placeholder.com/150"}, u, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select 1 user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url"}).
			AddRow("1", "Vikash", "http://via.placeholder.com/150"))
	setupMetrics(t, db, gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	u := user{}
	db.Select(t.Context(), &u, "select 1 user")

	assert.Equal(t, user{Name: "Vikash", ID: 1, Image: "http://via.placeholder.com/150"}, u, "TEST Failed.\n")
}

func TestDB_SelectMultiRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectQuery("select users").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url"}).
			AddRow("1", "Vikash", "http://via.placeholder.com/150").
			AddRow("2", "Gofr", ""))
	setupMetrics(t, db, gomock.Any())

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	users := []user{}
	db.Select(t.Context(), &users, "select users")

	assert.Equal(t, []user{
		{Name: "Vikash", ID: 1, Image: "http://via.placeholder.com/150"},
		{Name: "Gofr", ID: 2},
	}, users, "TEST Failed.\n")
}

func TestDB_SelectSliceRowsClosed(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mockRows := sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)
	mock.ExpectQuery("select id from users").WillReturnRows(mockRows)

	setupMetrics(t, db, gomock.Any())

	ids := make([]int, 0)
	db.Select(t.Context(), &ids, "select id from users")

	assert.NoError(t, mock.ExpectationsWereMet(), "rows were not closed after Select on slice")
}

func TestDB_SelectStructRowsClosed(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	type user struct {
		ID   int
		Name string
	}

	mockRows := sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Alice")
	mock.ExpectQuery("select from users").WillReturnRows(mockRows)
	setupMetrics(t, db, gomock.Any())

	u := user{}
	db.Select(t.Context(), &u, "select from users")

	assert.NoError(t, mock.ExpectationsWereMet(), "rows were not closed after Select on struct")
}

func TestDB_SelectSliceRowsIterationError(t *testing.T) {
	rowIterErr := testutil.CustomError{ErrorMessage: "row iteration error"}

	out := testutil.StderrOutputForFunc(func() {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		mock.ExpectQuery("select id from users").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2).RowError(1, rowIterErr))
		setupMetrics(t, db, gomock.Any())

		ids := make([]int, 0)
		db.Select(t.Context(), &ids, "select id from users")
	})

	assert.Contains(t, out, "row iteration error", "rows.Err() value should be logged, not the query error")
}

func TestDB_SelectStructRowsIterationError(t *testing.T) {
	rowIterErr := testutil.CustomError{ErrorMessage: "row iteration error"}

	out := testutil.StderrOutputForFunc(func() {
		db, mock := getDB(t, logging.INFO)
		defer db.DB.Close()

		type user struct {
			ID   int
			Name string
		}

		mock.ExpectQuery("select from users").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Alice").AddRow(2, "Bob").RowError(1, rowIterErr))
		setupMetrics(t, db, gomock.Any())

		u := user{}
		db.Select(t.Context(), &u, "select from users")
	})

	assert.Contains(t, out, "row iteration error", "rows.Err() value should be logged, not the query error")
}

// --- DB method tests ---

func TestDB_Query(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name:  "success",
			query: "SELECT 1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
			},
			logContains: "Query SELECT 1",
		},
		{
			name:  "error",
			query: "SELECT",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT ").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "Query SELECT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "SELECT")

				tc.setupMock(mock)

				rows, err := db.Query(tc.query)
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, rows)
				} else {
					require.NoError(t, err)
					require.NoError(t, rows.Err())
					assert.NotNil(t, rows)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestDB_QueryContext(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name:  "success",
			query: "SELECT 1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
			},
			logContains: "QueryContext SELECT 1",
		},
		{
			name:  "error",
			query: "SELECT",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT ").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "QueryContext SELECT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "SELECT")

				tc.setupMock(mock)

				rows, err := db.QueryContext(t.Context(), tc.query)
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, rows)
				} else {
					require.NoError(t, err)
					require.NoError(t, rows.Err())
					assert.NotNil(t, rows)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestDB_QueryRow(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		setupMetrics(t, db, "SELECT")

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("jhon"))

		row := db.QueryRow("SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRow SELECT name FROM employee WHERE id = ?")
}

func TestDB_QueryRowContext(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		setupMetrics(t, db, "SELECT")

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1)

		row := db.QueryRowContext(t.Context(), "SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRowContext SELECT name FROM employee WHERE id = ?")
}

func TestDB_Exec(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name:  "success",
			query: "INSERT INTO employee VALUES(?, ?)",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			logContains: "Exec INSERT INTO employee VALUES(?, ?)",
		},
		{
			name:  "error",
			query: "INSERT INTO employee VALUES(?, ?",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?").
					WithArgs(2, "doe").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "Exec INSERT INTO employee VALUES(?, ?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "INSERT")

				tc.setupMock(mock)

				res, err := db.Exec(tc.query, 2, "doe")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, res)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, res)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestDB_ExecContext(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name: "success",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			logContains: "ExecContext INSERT INTO employee VALUES(?, ?)",
		},
		{
			name: "error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "ExecContext INSERT INTO employee VALUES(?, ?)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "INSERT")

				tc.setupMock(mock)

				res, err := db.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, res)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, res)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestDB_Prepare(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name: "success",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
			},
			logContains: "Prepare SELECT name FROM employee WHERE id = ?",
		},
		{
			name: "error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "Prepare SELECT name FROM employee WHERE id = ?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "SELECT")

				tc.setupMock(mock)

				stmt, err := db.Prepare("SELECT name FROM employee WHERE id = ?")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, stmt)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, stmt)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestDB_Begin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{
			name:  "success",
			setup: func(mock sqlmock.Sqlmock) { mock.ExpectBegin() },
		},
		{
			name:    "error",
			setup:   func(mock sqlmock.Sqlmock) { mock.ExpectBegin().WillReturnError(errBegin) },
			wantErr: errBegin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := getDB(t, logging.INFO)
			defer db.DB.Close()

			tc.setup(mock)

			tx, err := db.Begin()
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, tx)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tx)
			}
		})
	}
}

func TestDB_BeginTx(t *testing.T) {
	db, mock := getDB(t, logging.DEBUG)
	defer db.DB.Close()

	db.metrics = NewMockMetrics(gomock.NewController(t))

	mock.ExpectBegin()

	tx, err := db.BeginTx(t.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	require.NoError(t, err)
	assert.NotNil(t, tx)
}

func TestDB_Close(t *testing.T) {
	db, mock := getDB(t, logging.INFO)

	mock.ExpectClose()

	err := db.Close()

	require.NoError(t, err)
}

func TestDB_CloseWhenNil(t *testing.T) {
	db := &DB{stopSignal: make(chan struct{})}
	assert.NoError(t, db.Close())
}

func TestDB_Dialect(t *testing.T) {
	db, _ := getDB(t, logging.INFO)
	defer db.Close()

	db.config.Dialect = "postgresql"
	require.Equal(t, "postgresql", db.Dialect())
}

func TestDB_Ping(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing()
				mock.ExpectPing().WillReturnError(nil)
			},
		},
		{
			name:    "failure",
			setup:   func(mock sqlmock.Sqlmock) { mock.ExpectPing().WillReturnError(sql.ErrConnDone) },
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := getDB(t, logging.DEBUG)
			defer db.DB.Close()

			db.metrics = NewMockMetrics(gomock.NewController(t))

			tc.setup(mock)

			err := db.PingContext(t.Context())
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDB_sendOperationStats_RecordsMilliseconds(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)

	db := &DB{
		logger:     logging.NewMockLogger(logging.DEBUG),
		config:     &DBConfig{HostName: "host", Database: "db"},
		metrics:    mockMetrics,
		stopSignal: make(chan struct{}),
	}

	start := time.Now().Add(-1500 * time.Millisecond) // 1.5 seconds ago

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), "app_sql_stats", float64(1500),
		"hostname", "host", "database", "db", "type", "SELECT",
	)

	db.sendOperationStats(start, "SELECT", "SELECT * FROM users")

	duration := time.Since(start).Milliseconds()
	assert.Equal(t, int64(1500), duration)
}

// --- Tx method tests ---

func TestTx_Query(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name:  "success",
			query: "SELECT 1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))
			},
			logContains: "Query SELECT 1",
		},
		{
			name:  "error",
			query: "SELECT",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT ").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "Query SELECT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "SELECT")

				tx := getTransaction(db, mock)
				tc.setupMock(mock)

				rows, err := tx.Query(tc.query)
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, rows)
				} else {
					require.NoError(t, err)
					require.NoError(t, rows.Err())
					assert.NotNil(t, rows)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestTx_QueryRow(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		setupMetrics(t, db, "SELECT")

		tx := getTransaction(db, mock)

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("jhon"))

		row := tx.QueryRow("SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRow SELECT name FROM employee WHERE id = ?")
}

func TestTx_QueryRowContext(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		db, mock := getDB(t, logging.DEBUG)
		defer db.DB.Close()

		setupMetrics(t, db, "SELECT")

		tx := getTransaction(db, mock)

		mock.ExpectQuery("SELECT name FROM employee WHERE id = ?").WithArgs(1)

		row := tx.QueryRowContext(t.Context(), "SELECT name FROM employee WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	assert.Contains(t, out, "QueryRowContext SELECT name FROM employee WHERE id = ?")
}

func TestTx_Exec(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name:  "success",
			query: "INSERT INTO employee VALUES(?, ?)",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			logContains: "TxExec INSERT INTO employee VALUES(?, ?)",
		},
		{
			name:  "error",
			query: "INSERT INTO employee VALUES(?, ?",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?").
					WithArgs(2, "doe").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "TxExec INSERT INTO employee VALUES(?, ?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "INSERT")

				tx := getTransaction(db, mock)
				tc.setupMock(mock)

				res, err := tx.Exec(tc.query, 2, "doe")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, res)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, res)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestTx_ExecContext(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name: "success",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			logContains: "ExecContext INSERT INTO employee VALUES(?, ?)",
		},
		{
			name: "error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO employee VALUES(?, ?)").
					WithArgs(2, "doe").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "ExecContext INSERT INTO employee VALUES(?, ?)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "INSERT")

				tx := getTransaction(db, mock)
				tc.setupMock(mock)

				res, err := tx.ExecContext(t.Context(), "INSERT INTO employee VALUES(?, ?)", 2, "doe")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, res)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, res)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestTx_Prepare(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		logContains string
	}{
		{
			name: "success",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?")
			},
			logContains: "Prepare SELECT name FROM employee WHERE id = ?",
		},
		{
			name: "error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPrepare("SELECT name FROM employee WHERE id = ?").WillReturnError(errSyntax)
			},
			wantErr:     true,
			logContains: "Prepare SELECT name FROM employee WHERE id = ?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "SELECT")

				tx := getTransaction(db, mock)
				tc.setupMock(mock)

				stmt, err := tx.Prepare("SELECT name FROM employee WHERE id = ?")
				if tc.wantErr {
					require.Error(t, err)
					assert.Nil(t, stmt)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, stmt)
				}
			})

			assert.Contains(t, out, tc.logContains)
		})
	}
}

func TestTx_Commit(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{
			name:  "success",
			setup: func(mock sqlmock.Sqlmock) { mock.ExpectCommit() },
		},
		{
			name:    "error",
			setup:   func(mock sqlmock.Sqlmock) { mock.ExpectCommit().WillReturnError(errDB) },
			wantErr: errDB,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "COMMIT")

				tx := getTransaction(db, mock)
				tc.setup(mock)

				err := tx.Commit()
				if tc.wantErr != nil {
					require.Error(t, err)
					assert.Equal(t, tc.wantErr, err)
				} else {
					require.NoError(t, err)
				}
			})

			assert.Contains(t, out, "TxCommit COMMIT")
		})
	}
}

func TestTx_Rollback(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{
			name:  "success",
			setup: func(mock sqlmock.Sqlmock) { mock.ExpectRollback() },
		},
		{
			name:    "error",
			setup:   func(mock sqlmock.Sqlmock) { mock.ExpectRollback().WillReturnError(errDB) },
			wantErr: errDB,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				db, mock := getDB(t, logging.DEBUG)
				defer db.DB.Close()

				setupMetrics(t, db, "ROLLBACK")

				tx := getTransaction(db, mock)
				tc.setup(mock)

				err := tx.Rollback()
				if tc.wantErr != nil {
					require.Error(t, err)
					assert.Equal(t, tc.wantErr, err)
				} else {
					require.NoError(t, err)
				}
			})

			assert.Contains(t, out, "TxRollback ROLLBACK")
		})
	}
}

func TestTx_Exec_SafeWithNilMetrics(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	db.metrics = nil

	mock.ExpectBegin()

	tx, err := db.Begin()
	require.NoError(t, err)

	mock.ExpectExec("INSERT INTO users VALUES(.*)").
		WillReturnResult(sqlmock.NewResult(1, 1))

	assert.NotPanics(t, func() {
		_, err = tx.Exec("INSERT INTO users VALUES(.*)", 1, "test")
	}, "Tx.Exec should NOT panic when metrics are nil")

	assert.NoError(t, err)
}

// --- Utility tests ---

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

func TestGetOperationType(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"SELECT * FROM users", "SELECT"},
		{"  SELECT * FROM users", "SELECT"},
		{"  INSERT INTO users", "INSERT"},
		{"UPDATE users SET name = ?", "UPDATE"},
		{"DELETE FROM users", "DELETE"},
		{"", ""},
		{"   ", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, getOperationType(tc.query))
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"  SELECT  ", "SELECT"},
		{"SELECT   *   FROM   users", "SELECT * FROM users"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, clean(tc.input))
	}
}
