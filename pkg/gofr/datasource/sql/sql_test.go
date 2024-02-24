package sql

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
)

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
		configs DBConfig
		expOut  string
		expErr  error
	}{
		{
			desc: "mysql dialect",
			configs: DBConfig{
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
			configs: DBConfig{
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
			desc:    "unsupported dialect",
			configs: DBConfig{Dialect: "mssql"},
			expOut:  "",
			expErr:  errUnsupportedDialect,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			connString, err := getDBConnectionString(&tc.configs)

			assert.Equal(t, tc.expOut, connString)
			assert.Equal(t, tc.expErr, err)
		})
	}
}
