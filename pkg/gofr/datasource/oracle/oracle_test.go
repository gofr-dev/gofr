package oracle

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

var (
	errExecTest      = errors.New("exec err")
	errSelectTest    = errors.New("select err")
	errPingTest      = errors.New("ping error")
	errTableNotExist = errors.New("ORA-00942: table or view does not exist")
	errSomeTest      = errors.New("some error")
	errQueryTest     = errors.New("query error")
)

func getOracleTestConnection(t *testing.T) (*MockConnection, *MockLogger, Client) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockConn := NewMockConnection(ctrl)
	mockMetric := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	c := Client{conn: mockConn, config: Config{
		Host:     "localhost",
		Port:     1521,
		Username: "system",
		Password: "password",
		Service:  "FREEPDB1",
	}, logger: mockLogger, metrics: mockMetric}

	return mockConn, mockLogger, c
}

func Test_Oracle_HealthUP(t *testing.T) {
	mockConn, _, c := getOracleTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(nil)

	resp, _ := c.HealthCheck(t.Context())

	health, ok := resp.(*Health)

	require.True(t, ok)

	assert.Equal(t, "UP", health.Status)
}

func Test_Oracle_HealthDOWN(t *testing.T) {
	mockConn, _, c := getOracleTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(sql.ErrConnDone)

	resp, err := c.HealthCheck(t.Context())

	require.ErrorIs(t, err, errStatusDown)

	health, ok := resp.(*Health)

	require.True(t, ok)

	assert.Equal(t, "DOWN", health.Status)
}

func Test_Oracle_Exec(t *testing.T) {
	mockConn, mockLogger, c := getOracleTestConnection(t)

	ctx := t.Context()

	mockConn.EXPECT().Exec(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", 1, "user").Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())

	err := c.Exec(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", 1, "user")

	require.NoError(t, err)
}

func Test_Oracle_Select(t *testing.T) {
	mockConn, mockLogger, c := getOracleTestConnection(t)

	type User struct {
		ID   int
		Name string
	}

	ctx := t.Context()

	var users []User

	mockConn.EXPECT().Select(ctx, &users, "SELECT * FROM users").Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())

	err := c.Select(ctx, &users, "SELECT * FROM users")

	require.NoError(t, err)
}

func Test_New_ReturnsClient(t *testing.T) {
	cfg := Config{Host: "h", Port: 1, Username: "u", Password: "p", Service: "s"}

	c := New(cfg)

	require.NotNil(t, c)

	assert.Equal(t, cfg, c.config)
}

func Test_UseLogger_SetsLoggerWhenCorrectType(t *testing.T) {
	c := New(Config{})

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	c.UseLogger(mockLog)

	assert.Equal(t, mockLog, c.logger)

	c.UseLogger("not a logger")
	// logger should remain unchanged.
	assert.Equal(t, mockLog, c.logger)
}

func Test_UseMetrics_SetsMetricsWhenCorrectType(t *testing.T) {
	c := New(Config{})

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	c.UseMetrics(mockMetrics)

	assert.Equal(t, mockMetrics, c.metrics)

	c.UseMetrics(123) // ignored.

	assert.Equal(t, mockMetrics, c.metrics)
}

func Test_UseTracer_SetsTracerWhenCorrectType(t *testing.T) {
	c := New(Config{})

	tracerMock := noop.NewTracerProvider().Tracer("test") // or custom mock.

	c.UseTracer(tracerMock)

	assert.Equal(t, tracerMock, c.tracer)

	c.UseTracer("wrong")
	// Should ignore, tracer remains tracerMock.
	assert.Equal(t, tracerMock, c.tracer)
}

func Test_Connect_SuccessAndFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{Host: "h", Port: 1, Username: "u", Password: "p", Service: "s"})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	// --- Fail sql.Open ---
	c.config.Username = "baduser"

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	// --- Success ---
	c.config.Username = "system"

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Logf(gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	c.Connect()

	require.NotNil(t, c.conn)
}

func Test_Exec_ErrorPropagation(t *testing.T) {
	mockConn, mockLogger, c := getOracleTestConnection(t)

	ctx := t.Context()

	mockLogger.EXPECT().Debug(gomock.Any())

	mockConn.EXPECT().Exec(ctx, "QUERY", gomock.Any()).Return(errExecTest)

	err := c.Exec(ctx, "QUERY", 123)

	require.Error(t, err)

	assert.Contains(t, err.Error(), errExecTest.Error())
}

func Test_Select_InvalidDestType(t *testing.T) {
	mockConn, _, c := getOracleTestConnection(t)

	mockConn.EXPECT().Select(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := c.Select(t.Context(), "invalid-type", "SELECT 1")

	require.Equal(t, errInvalidDestType, err)
}

func Test_Select_ErrorPropagation(t *testing.T) {
	mockConn, mockLogger, c := getOracleTestConnection(t)

	ctx := t.Context()

	mockLogger.EXPECT().Debug(gomock.Any())

	mockConn.EXPECT().Select(ctx, gomock.Any(), "QUERY", gomock.Any()).Return(errSelectTest)

	var result []map[string]any

	err := c.Select(ctx, &result, "QUERY", 123)

	require.Error(t, err)

	assert.Contains(t, err.Error(), errSelectTest.Error())
}

func Test_addTrace_WithAndWithoutTracer(t *testing.T) {
	c := New(Config{})

	ctx := t.Context()

	ctx2, span := c.addTrace(ctx, "method", "query")

	assert.Nil(t, span)

	assert.Equal(t, ctx, ctx2)

	tracerMock := noop.NewTracerProvider().Tracer("test")

	c.UseTracer(tracerMock)

	ctx3, span2 := c.addTrace(ctx, "method", "query")

	require.NotNil(t, span2)

	span2.End() // manually end.

	assert.NotEqual(t, ctx, ctx3) // ctx with span.
}

func Test_sendOperationStats_WithAndWithoutSpan(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	mockMetrics := NewMockMetrics(ctrl)

	c := New(Config{})

	c.UseLogger(mockLogger)

	c.UseMetrics(mockMetrics)

	mockLogger.EXPECT().Debug(gomock.Any())

	// span == nil case; no call to span.End().
	c.sendOperationStats(time.Now(), "Exec", "SELECT 1", "exec", nil)

	// With mock span.
	tracer := noop.NewTracerProvider().Tracer("test")

	c.UseTracer(tracer)

	_, span := c.addTrace(t.Context(), "exec", "SELECT 1")

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	c.sendOperationStats(time.Now(), "Exec", "SELECT 1", "exec", span)
}

func Test_Ping_ReturnsErrorOrNil(t *testing.T) {
	mockConn, _, c := getOracleTestConnection(t)

	ctx := t.Context()

	mockConn.EXPECT().Ping(ctx).Return(nil)

	err := c.conn.Ping(ctx)

	require.NoError(t, err)

	mockConn.EXPECT().Ping(ctx).Return(errPingTest)

	err = c.conn.Ping(ctx)

	require.Error(t, err)
}

func Test_Stats_ReturnsValue(t *testing.T) {
	mockConn, _, c := getOracleTestConnection(t)

	mockConn.EXPECT().Stats().Return("stats")

	result := c.conn.Stats()

	assert.Equal(t, "stats", result)
}

func Test_LoggingWithDebugf_Errorf_Logf(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debugf("debug pattern %s", gomock.Any())
	mockLogger.EXPECT().Errorf("error pattern %s", gomock.Any())
	mockLogger.EXPECT().Logf("log pattern %s", gomock.Any())

	mockLogger.Debugf("debug pattern %s", "arg")
	mockLogger.Errorf("error pattern %s", "arg")
	mockLogger.Logf("log pattern %s", "arg")
}

func Test_MetricsCalls(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	ctx := t.Context()

	mockMetrics.EXPECT().NewHistogram("name", "desc", gomock.Any()).Times(1)
	mockMetrics.EXPECT().NewGauge("gauge", "desc").Times(1)
	mockMetrics.EXPECT().RecordHistogram(ctx, "hist", float64(123), "label").Times(1)
	mockMetrics.EXPECT().SetGauge("gauge", float64(456), "label").Times(1)

	mockMetrics.NewHistogram("name", "desc", 0.1, 1.0)
	mockMetrics.NewGauge("gauge", "desc")
	mockMetrics.RecordHistogram(ctx, "hist", 123, "label")
	mockMetrics.SetGauge("gauge", 456, "label")
}

func Test_sqlConn_Exec(t *testing.T) {
	db, mock, err := sqlmock.New()

	require.NoError(t, err)

	defer db.Close()

	s := &sqlConn{db: db}

	mock.ExpectExec("INSERT INTO users").WithArgs(1, "gofr").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = s.Exec(t.Context(), "INSERT INTO users (id, name) VALUES (?, ?)", 1, "gofr")

	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func Test_sqlConn_Select(t *testing.T) {
	db, mock, err := sqlmock.New()

	require.NoError(t, err)

	defer db.Close()

	s := &sqlConn{db: db}

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "gofr").
		AddRow(2, "dev")

	mock.ExpectQuery("SELECT id, name FROM users").WillReturnRows(rows)

	var result []map[string]any

	err = s.Select(t.Context(), &result, "SELECT id, name FROM users")

	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "gofr", result[0]["name"])
	assert.Equal(t, int64(1), result[0]["id"])
	assert.Equal(t, "dev", result[1]["name"])
	assert.Equal(t, int64(2), result[1]["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func Test_sqlConn_Stats(t *testing.T) {
	db, _, err := sqlmock.New()

	require.NoError(t, err)

	defer db.Close()

	s := &sqlConn{db: db}

	stats := s.Stats()

	// Check that it returns non-nil and is of correct type.
	_, ok := stats.(sql.DBStats)

	assert.True(t, ok)
}

func Test_Oracle_InvalidHostName(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{
		Host:     "invalid.hostname",
		Port:     1521,
		Username: "system",
		Password: "password",
		Service:  "FREEPDB1",
	})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	c.Connect()
}

func Test_sqlConn_InvalidInsertQuery(t *testing.T) {
	db, mock, err := sqlmock.New()

	require.NoError(t, err)

	defer db.Close()

	s := &sqlConn{db: db}

	mock.ExpectExec("INSERT INTO bad_table").WillReturnError(errTableNotExist)

	err = s.Exec(t.Context(), "INSERT INTO bad_table (id) VALUES (?)", 1)

	require.Error(t, err)

	assert.Contains(t, err.Error(), "table or view does not exist")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func Test_Oracle_ConnectionTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{
		Host:     "10.255.255.1", // unreachable IP
		Port:     1521,
		Username: "system",
		Password: "password",
		Service:  "FREEPDB1",
	})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	c.Connect()
}

func Test_Oracle_ConnectionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{
		Host:     "localhost",
		Port:     1521,
		Username: "wrong_user",
		Password: "wrong_pass",
		Service:  "FREEPDB1",
	})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	c.Connect()
}

func Test_Connect_InvalidHost(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{Host: "", Port: 1521, Username: "u", Password: "p", Service: "s"})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	mockLogger.EXPECT().Errorf("invalid OracleDB host: host is empty")

	c.Connect()
}

func Test_Connect_InvalidPort(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	c := New(Config{Host: "h", Port: 0, Username: "u", Password: "p", Service: "s"})

	mockLogger := NewMockLogger(ctrl)

	c.UseLogger(mockLogger)

	mockLogger.EXPECT().Errorf("invalid OracleDB port: %v", 0)

	c.Connect()
}

func Test_sqlConn_Exec_Errors(t *testing.T) {
	db, mock, _ := sqlmock.New()

	defer db.Close()

	s := &sqlConn{db: db}

	mock.ExpectExec("BAD QUERY").WillReturnError(errSomeTest)

	err := s.Exec(t.Context(), "BAD QUERY")

	require.Error(t, err)
}

func Test_sqlConn_Select_ColumnsError(t *testing.T) {
	db, mock, _ := sqlmock.New()

	defer db.Close()

	s := &sqlConn{db: db}

	mock.ExpectQuery("SELECT").WillReturnError(errQueryTest)

	var dest []map[string]any

	err := s.Select(t.Context(), &dest, "SELECT * FROM dual")

	require.Error(t, err)
}

func Test_sqlConn_Ping(t *testing.T) {
	db, _, _ := sqlmock.New()

	defer db.Close()

	s := &sqlConn{db: db}

	err := s.Ping(t.Context())

	require.NoError(t, err)
}

func Test_sqlConn_Stats_ReturnsDBStats(t *testing.T) {
	db, _, _ := sqlmock.New()

	defer db.Close()

	s := &sqlConn{db: db}

	stats := s.Stats()

	_, ok := stats.(sql.DBStats)

	assert.True(t, ok)
}
