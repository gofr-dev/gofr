package sql

import (
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestNewSQL_ErrorCase(t *testing.T) {
	ctrl := gomock.NewController(t)

	expectedLog := "could not connect with 'testuser' user to database 'localhost:3306'  error"

	mockConfig := testutil.NewMockConfig(map[string]string{
		"DB_DIALECT":  "mysql",
		"DB_HOST":     "localhost",
		"DB_USER":     "testuser",
		"DB_PASSWORD": "testpassword",
		"DB_PORT":     "3306",
		"DB_NAME":     "testdb",
	})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		mockMetrics := NewMockMetrics(ctrl)

		NewSQL(mockConfig, mockLogger, mockMetrics)
	})

	if !strings.Contains(testLogs, expectedLog) {
		t.Errorf("TestNewSQL_ErrorCase Failed! Expcted error log doesn't match actual.")
	}
}

func TestNewSQL_InvalidDialect(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := testutil.NewMockConfig(map[string]string{
		"DB_DIALECT": "abc",
		"DB_HOST":    "localhost",
	})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		mockMetrics := NewMockMetrics(ctrl)

		NewSQL(mockConfig, mockLogger, mockMetrics)
	})

	if !strings.Contains(testLogs, errUnsupportedDialect.Error()) {
		t.Errorf("TestNewSQL_ErrorCase Failed! Expcted error log doesn't match actual.")
	}
}

func TestNewSQL_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := testutil.NewMockConfig(map[string]string{
		"DB_DIALECT": "",
	})

	mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
	mockMetrics := NewMockMetrics(ctrl)

	db := NewSQL(mockConfig, mockLogger, mockMetrics)

	assert.Nil(t, db, "TestNewSQL_InvalidConfig. expected db to be nil.")
}

func TestSQL_GetDBConfig(t *testing.T) {
	mockConfig := testutil.NewMockConfig(map[string]string{
		"DB_DIALECT":  "mysql",
		"DB_HOST":     "host",
		"DB_USER":     "user",
		"DB_PASSWORD": "password",
		"DB_PORT":     "3201",
		"DB_NAME":     "test",
	})

	expectedComfigs := &DBConfig{
		Dialect:  "mysql",
		HostName: "host",
		User:     "user",
		Password: "password",
		Port:     "3201",
		Database: "test",
	}

	configs := getDBConfig(mockConfig)

	assert.Equal(t, expectedComfigs, configs)
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
			},
			expOut: "host=host port=3201 user=user password=password dbname=test sslmode=disable",
		},
		{
			desc: "sqlite dialect",
			configs: &DBConfig{
				Dialect:  "sqlite",
				HostName: "file:test.db?cache=shared&mode=memory",
			},
			expOut: "file:test.db?cache=shared&mode=memory",
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
