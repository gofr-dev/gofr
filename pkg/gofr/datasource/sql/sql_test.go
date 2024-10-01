package sql

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewSQL_ErrorCase(t *testing.T) {
	ctrl := gomock.NewController(t)

	expectedLog := fmt.Sprintf("could not register sql dialect '%s' for traces, error: %s", "mysql",
		"sql: unknown driver \"mysql\" (forgotten import?)")

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "mysql",
		"DB_HOST":     "localhost",
		"DB_USER":     "testuser",
		"DB_PASSWORD": "testpassword",
		"DB_PORT":     "3306",
		"DB_NAME":     "testdb",
	})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.ERROR)
		mockMetrics := NewMockMetrics(ctrl)

		NewSQL(mockConfig, mockLogger, mockMetrics)
	})

	assert.Containsf(t, testLogs, expectedLog, "TestNewSQL_ErrorCase Failed! Expected error log doesn't match actual.")
}

func TestNewSQL_InvalidDialect(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT": "abc",
		"DB_HOST":    "localhost",
	})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.ERROR)
		mockMetrics := NewMockMetrics(ctrl)

		NewSQL(mockConfig, mockLogger, mockMetrics)
	})

	assert.Containsf(t, testLogs, errUnsupportedDialect.Error(), "TestNewSQL_ErrorCase Failed! Expected error log doesn't match actual.")
}

func TestNewSQL_GetDBDialect(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT": "postgres",
		"DB_HOST":    "localhost",
	})

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockMetrics := NewMockMetrics(ctrl)

	// using gomock.Any as we are not actually testing any feature related to metrics
	mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

	db := NewSQL(mockConfig, mockLogger, mockMetrics)

	dialect := db.Dialect()

	assert.Equal(t, "postgres", dialect)

	time.Sleep(100 * time.Millisecond)
}

func TestNewSQL_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT": "",
	})

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockMetrics := NewMockMetrics(ctrl)

	db := NewSQL(mockConfig, mockLogger, mockMetrics)

	assert.Nil(t, db, "TestNewSQL_InvalidConfig. expected db to be nil.")
}

func TestSQL_GetDBConfig(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT":             "mysql",
		"DB_HOST":                "host",
		"DB_USER":                "user",
		"DB_PASSWORD":            "password",
		"DB_PORT":                "3201",
		"DB_NAME":                "test",
		"DB_SSL_MODE":            "require",
		"DB_MAX_IDLE_CONNECTION": "25",
		"DB_MAX_OPEN_CONNECTION": "50",
	})

	expectedComfigs := &DBConfig{
		Dialect:     "mysql",
		HostName:    "host",
		User:        "user",
		Password:    "password",
		Port:        "3201",
		Database:    "test",
		SSLMode:     "require",
		MaxIdleConn: 25,
		MaxOpenConn: 50,
	}

	configs := getDBConfig(mockConfig)

	assert.Equal(t, expectedComfigs, configs)
}

func TestSQL_ConfigCases(t *testing.T) {
	testCases := []struct {
		name         string
		idleConn     string
		openConn     string
		expectedIdle int
		expectedOpen int
	}{
		{
			name:         "Invalid Max Idle and Open Connections",
			idleConn:     "abc",
			openConn:     "def",
			expectedIdle: 2,
			expectedOpen: 0,
		},
		{
			name:         "Negative Max Idle and Open Connections",
			idleConn:     "-2",
			openConn:     "-3",
			expectedIdle: -2,
			expectedOpen: -3,
		},
	}

	for _, tc := range testCases {
		mockConfig := config.NewMockConfig(map[string]string{
			"DB_MAX_IDLE_CONNECTION": tc.idleConn,
			"DB_MAX_OPEN_CONNECTION": tc.openConn,
		})

		expectedConfig := &DBConfig{
			Port:        "3306",
			MaxIdleConn: tc.expectedIdle,
			MaxOpenConn: tc.expectedOpen,
			SSLMode:     "disable",
		}

		configs := getDBConfig(mockConfig)

		assert.Equal(t, expectedConfig, configs)
	}
}

func TestSQL_getDBConnectionString(t *testing.T) {
	testCases := []struct {
		desc    string
		configs *DBConfig
		expOut  string
		expErr  error
	}{
		{
			desc: "mysql dialect",
			configs: &DBConfig{
				Dialect:  "mysql",
				HostName: "host",
				User:     "user",
				Password: "password",
				Port:     "3201",
				Database: "test",
			},
			expOut: "user:password@tcp(host:3201)/test?charset=utf8&parseTime=True&loc=Local&interpolateParams=true",
		},
		{
			desc: "postgresql dialect",
			configs: &DBConfig{
				Dialect:  "postgres",
				HostName: "host",
				User:     "user",
				Password: "password",
				Port:     "3201",
				Database: "test",
				SSLMode:  "require",
			},
			expOut: "host=host port=3201 user=user password=password dbname=test sslmode=require",
		},
		{
			desc: "postgresql dialect",
			configs: &DBConfig{
				Dialect:  "postgres",
				HostName: "host",
				User:     "user",
				Password: "password",
				Port:     "3201",
				Database: "test",
				SSLMode:  "disable",
			},
			expOut: "host=host port=3201 user=user password=password dbname=test sslmode=disable",
		},
		{
			desc: "sqlite dialect",
			configs: &DBConfig{
				Dialect:  "sqlite",
				Database: "test.db",
			},
			expOut: "file:test.db",
		},
		{
			desc:    "unsupported dialect",
			configs: &DBConfig{Dialect: "mssql"},
			expOut:  "",
			expErr:  errUnsupportedDialect,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			connString, err := getDBConnectionString(tc.configs)

			assert.Equal(t, tc.expOut, connString)
			assert.Equal(t, tc.expErr, err)
		})
	}
}

func Test_NewSQLMock(t *testing.T) {
	db, mock, mockMetric := NewSQLMocks(t)

	assert.NotNil(t, db)
	assert.NotNil(t, mock)
	assert.NotNil(t, mockMetric)
}

func Test_NewSQLMockWithConfig(t *testing.T) {
	dbConfig := DBConfig{Dialect: "dialect", HostName: "hostname", User: "user", Password: "password", Port: "port", Database: "database"}
	db, mock, mockMetric := NewSQLMocksWithConfig(t, &dbConfig)

	assert.NotNil(t, db)
	assert.Equal(t, db.config, &dbConfig)
	assert.NotNil(t, mock)
	assert.NotNil(t, mockMetric)
}

var errSqliteConnection = errors.New("connection failed")

func Test_sqliteSuccessfulConnLogs(t *testing.T) {
	tests := []struct {
		desc        string
		status      string
		expectedLog string
	}{
		{"sqlite connection in process", "connecting", `connecting to 'test' database`},
		{"sqlite connected successfully", "connected", `connected to 'test' database`},
	}

	for _, test := range tests {
		logs := testutil.StdoutOutputForFunc(func() {
			mockLogger := logging.NewMockLogger(logging.DEBUG)
			mockConfig := &DBConfig{
				Dialect:  sqlite,
				Database: "test",
			}

			printConnectionSuccessLog(test.status, mockConfig, mockLogger)
		})

		assert.Contains(t, logs, test.expectedLog)
	}
}

func Test_sqliteErrConnLogs(t *testing.T) {
	test := []struct {
		desc        string
		action      string
		err         error
		expectedLog string
	}{
		{"sqlite connection failure", "connect", errSqliteConnection,
			`could not connect database 'test', error: connection failed`},
		{"sqlite open connection failure", "open connection with", errSqliteConnection,
			`could not open connection with database 'test', error: connection failed`},
	}
	for _, tt := range test {
		logs := testutil.StderrOutputForFunc(func() {
			mockLogger := logging.NewMockLogger(logging.DEBUG)
			mockConfig := &DBConfig{
				Dialect:  sqlite,
				Database: "test",
			}

			printConnectionFailureLog(tt.action, mockConfig, mockLogger, tt.err)
		})

		assert.Contains(t, logs, tt.expectedLog)
	}
}

func Test_SQLRetryConnectionInfoLog(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)

		mockMetrics := NewMockMetrics(ctrl)
		mockConfig := config.NewMockConfig(map[string]string{
			"DB_DIALECT":  "postgres",
			"DB_HOST":     "host",
			"DB_USER":     "user",
			"DB_PASSWORD": "password",
			"DB_PORT":     "3201",
			"DB_NAME":     "test",
		})

		mockLogger := logging.NewMockLogger(logging.DEBUG)

		mockMetrics.EXPECT().SetGauge("app_sql_open_connections", float64(0))
		mockMetrics.EXPECT().SetGauge("app_sql_inUse_connections", float64(0))

		_ = NewSQL(mockConfig, mockLogger, mockMetrics)

		time.Sleep(100 * time.Millisecond)
	})

	assert.Contains(t, logs, "retrying SQL database connection")
}
