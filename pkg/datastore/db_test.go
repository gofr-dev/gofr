package datastore

import (
	"bytes"
	"net"
	"strings"
	"testing"

	go_sql_driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/log"
)

const logMsg = "Health check failed"

func TestNewORM(t *testing.T) {
	// failure case
	{
		_, err := NewORM(&DBConfig{
			HostName: "fake host",
			Username: "root",
			Password: "root123",
			Database: "mysql",
			Port:     "1000",
			Dialect:  "mysql",
		})

		e := new(net.DNSError)

		assert.NotEqual(t, err, &e, "\"FAILED, expected: %s, got: %s\", e, err")
	}

	//	failure case due to invalid client certificate path
	{
		dc := DBConfig{
			SSL:               "require",
			HostName:          "localhost",
			Username:          "root",
			Password:          "password",
			Database:          "mysql",
			Port:              "2001",
			Dialect:           "mysql",
			CertificateFile:   "../../.github/setups/certFiles/client-cert.pem",
			KeyFile:           "invalid key file",
			CACertificateFile: "../../.github/setups/certFiles/ca-cert.pem",
		}

		_, e := createSSLConfig(dc.CertificateFile, dc.KeyFile, dc.CACertificateFile)

		_, err := NewORM(&dc)
		if err != nil {
			assert.IsType(t, e, err, "Test failed ")
		}
	}

	// failure case due to invalid dialect
	{
		dc := DBConfig{
			Dialect: "fake dialect",
		}

		_, err := NewORM(&dc)

		assert.IsType(t, invalidDialect{}, err, "\"FAILED, expected: %s, got: %s\", e, err")
	}
}

func TestNewORM_Success(t *testing.T) {
	t.Skip("skipping tests in short mode")

	dc := DBConfig{
		SSL:               "require",
		HostName:          "localhost",
		Username:          "root",
		Password:          "password",
		Database:          "mysql",
		Port:              "2001",
		Dialect:           "mysql",
		CertificateFile:   "../../.github/setups/certFiles/client-cert.pem",
		KeyFile:           "../../.github/setups/certFiles/client-key.pem",
		CACertificateFile: "../../.github/setups/certFiles/ca-cert.pem",
	}

	db, err := NewORM(&dc)
	if err != nil {
		t.Errorf("FAILED, Could not connect to SQL, got error: %v\n", err)
		return
	}

	err = db.Exec("SELECT User FROM mysql.user").Error
	if err != nil {
		t.Errorf("FAILED, Could not run sql command, got error: %v\n", err)
	}
}

func TestInvalidDialect_Error(t *testing.T) {
	var err invalidDialect

	expected := "invalid dialect: supported dialects are - mysql, mssql, sqlite, postgres"

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}

func Test_formConnectionStr(t *testing.T) {
	cfg := DBConfig{
		HostName: "host",
		Database: "test",
		Port:     "1234",
	}

	tests := []struct {
		name     string
		username string
		sslMode  string
		dialect  string
		password string
		want     string
	}{
		{"postgres", "user", "", "postgres", "pass",
			"postgres://user@host:1234/test?password=pass&sslmode=disable&sslcert=&sslkey="},
		{"postgres", "user%", "", "postgres", "pass",
			"postgres://user%25@host:1234/test?password=pass&sslmode=disable&sslcert=&sslkey="},
		{"postgres", "user%", "", "postgres", "pass%",
			"postgres://user%25@host:1234/test?password=pass%25&sslmode=disable&sslcert=&sslkey="},
		{"postgres", "user{1}", "", "postgres", "pass{1}",
			`postgres://user%7B1%7D@host:1234/test?password=pass%7B1%7D&sslmode=disable&sslcert=&sslkey=`},
		{"postgres", "user%7B%7D", "", "postgres", "pass",
			"postgres://user%7B%7D@host:1234/test?password=pass&sslmode=disable&sslcert=&sslkey="},
		{"mssql", "user", "", "mssql", "pass",
			"sqlserver://user:pass@host:1234?database=test"},
		{"mssql", "user%7B%7D", "", "mssql", "pass",
			"sqlserver://user%7B%7D:pass@host:1234?database=test"},
		{"mssql", "user{2}", "", "mssql", "root{1}",
			"sqlserver://user%7B2%7D:root%7B1%7D@host:1234?database=test"},
		{"mssql", "user", "", "mssql", "root%",
			"sqlserver://user:root%25@host:1234?database=test"},
		{"mysql", "user", "", "mysql", "pass",
			"user:pass@tcp(host:1234)/test?charset=utf8&parseTime=True&loc=Local"},
		{"mysql", "user{3}", "", "mysql", "pass",
			"user%7B3%7D:pass@tcp(host:1234)/test?charset=utf8&parseTime=True&loc=Local"},
		{"mysql", "user%7B%7D", "", "mysql", "pass",
			"user%7B%7D:pass@tcp(host:1234)/test?charset=utf8&parseTime=True&loc=Local"},
		{"mysql", "user%7B%7D", "", "mysql", "pass{}",
			"user%7B%7D:pass{}@tcp(host:1234)/test?charset=utf8&parseTime=True&loc=Local"},
		{"mysql", "user%7B%7D", "require", "mysql", "pass{}",
			"user%7B%7D:pass{}@tcp(host:1234)/test?tls=custom&charset=utf8&parseTime=True&loc=Local"},
	}

	for i, tc := range tests {
		cfg.Dialect = tc.dialect
		cfg.Username = tc.username
		cfg.Password = tc.password
		cfg.SSL = tc.sslMode
		got := formConnectionStr(&cfg)

		assert.Equal(t, tc.want, got, "TEST[%v] failed\n%s", i, tc.name)
	}
}

func Test_queryEscape(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{
		{"user%", "user%25"},
		{"user{}", "user%7B%7D"},
		{"user", "user"},
		{"user%7B", "user%7B"},
	}
	for i, tc := range testCases {
		out := queryEscape(tc.input)
		assert.Equal(t, tc.output, out, "Test [%v] Failed.", i)
	}
}

func Test_NewSQLX(t *testing.T) {
	// failure case
	{
		_, err := NewSQLX(&DBConfig{
			HostName: "fake host",
			Username: "root",
			Password: "root123",
			Database: "mysql",
			Port:     "1000",
			Dialect:  "mysql",
		})

		e := new(net.DNSError)

		assert.NotEqual(t, err, &e, "FAILED, expected: %s, got: %s", e, err)
	}
}

func TestDataStore_SQL_SQLX_HealthCheck(t *testing.T) {
	t.Skip("skipping test in short mode")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	dc := DBConfig{
		SSL:               "require",
		HostName:          "localhost",
		Username:          "root",
		Password:          "password",
		Database:          "mysql",
		Port:              "2001",
		Dialect:           "mysql",
		CertificateFile:   "../../.github/setups/certFiles/client-cert.pem",
		KeyFile:           "../../.github/setups/certFiles/client-key.pem",
		CACertificateFile: "../../.github/setups/certFiles/ca-cert.pem",
	}

	testcases := []struct {
		host       string
		status     string
		logMessage string
	}{
		{dc.HostName, pkg.StatusUp, ""},
		{"invalid", pkg.StatusDown, logMsg},
	}

	for i, v := range testcases {
		dc.HostName = v.host

		clientSQL, _ := NewORM(&dc)
		clientSQL.logger = logger
		dsSQL := DataStore{gorm: clientSQL}

		healthCheck := dsSQL.SQLHealthCheck()
		if healthCheck.Status != v.status {
			t.Errorf("[TESTCASE%d]SQL Failed. Expected status: %v\n Got: %v", i+1, v.status, healthCheck)
		}

		if !strings.Contains(b.String(), v.logMessage) {
			t.Errorf("Test Failed \nExpected: %v\nGot: %v", v.logMessage, b.String())
		}

		// connecting to SQLX
		clientSQLX, _ := NewSQLX(&dc)
		dsSQLX := DataStore{sqlx: clientSQLX}

		healthCheck = dsSQLX.SQLXHealthCheck()
		if healthCheck.Status != v.status {
			t.Errorf("[TESTCASE%d]SQLX Failed. Expected status: %v\n Got: %v", i+1, v.status, healthCheck)
		}
	}
}

func TestNewSQLX_Success(t *testing.T) {
	t.Skip("skipping tests in short mode")

	{
		dc := DBConfig{
			SSL:               "require",
			HostName:          "localhost",
			Username:          "root",
			Password:          "password",
			Database:          "mysql",
			Port:              "2001",
			Dialect:           "mysql",
			CertificateFile:   "../../.github/setups/certFiles/client-cert.pem",
			KeyFile:           "../../.github/setups/certFiles/client-key.pem",
			CACertificateFile: "../../.github/setups/certFiles/ca-cert.pem",
		}

		sslConf, _ := createSSLConfig(dc.CACertificateFile, dc.CertificateFile, dc.KeyFile)

		err := go_sql_driver.RegisterTLSConfig("custom", sslConf)
		if err != nil {
			return
		}
		_, err = NewSQLX(&dc)
		if err != nil {
			t.Errorf("FAILED, expected: %v, got: %v", nil, err)
		}
	}
}

// Test_SQL_SQLX_HealthCheck_Down tests health check response when the db connection was made but lost in between
func Test_SQL_SQLX_HealthCheck_Down(t *testing.T) {
	t.Skip("skipping tests in short mode")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	dc := DBConfig{
		SSL:               "require",
		HostName:          "localhost",
		Username:          "root",
		Password:          "password",
		Database:          "mysql",
		Port:              "2001",
		Dialect:           "mysql",
		CertificateFile:   "../../.github/setups/certFiles/client-cert.pem",
		KeyFile:           "../../.github/setups/certFiles/client-key.pem",
		CACertificateFile: "../../.github/setups/certFiles/ca-cert.pem",
	}
	{
		clientSQL, _ := NewORM(&dc)
		clientSQL.logger = logger
		dsSQL := DataStore{gorm: clientSQL}

		db, _ := clientSQL.DB.DB()

		// db connected but goes down in between
		db.Close()

		healthCheck := dsSQL.SQLHealthCheck()
		if healthCheck.Status != pkg.StatusDown {
			t.Errorf("Failed. Expected: DOWN, Got: %v", healthCheck.Status)
		}

		if !strings.Contains(b.String(), logMsg) {
			t.Errorf("Test Failed \nExpected: %v\nGot: %v", logMsg, b.String())
		}
	}

	{
		// connecting to SQLX
		clientSQLX, _ := NewSQLX(&dc)
		dsSQLX := DataStore{sqlx: clientSQLX}

		// db connected but goes down in between
		clientSQLX.Close()

		healthCheck := dsSQLX.SQLXHealthCheck()
		if healthCheck.Status != pkg.StatusDown {
			t.Errorf("Test Failed. Expected: DOWN, Got: %v", healthCheck.Status)
		}
	}
}

func Test_DB_Credentials_Logging(t *testing.T) {
	cfg := &DBConfig{
		HostName: "localhost{}",
		Username: "user{}",
		Password: "root123{}",
		Database: "postgres",
		Port:     "2006",
		Dialect:  "postgres",
	}
	dialects := []string{"postgres", "mssql", "mysql", "sqlite"}

	for _, d := range dialects {
		cfg.Dialect = d

		_, err := NewORM(cfg)
		if strings.Contains(err.Error(), cfg.Password) {
			t.Errorf("Test case failed for %s. Password not expected in logs, got: %v", d, err.Error())
		}

		if strings.Contains(err.Error(), cfg.Username) {
			t.Errorf("Test case failed for %s. Username not expected in logs, got: %v", d, err.Error())
		}
	}
}

func TestNewORMFromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_NAME", "mysql")
	t.Setenv("DB_PORT", "2001")
	t.Setenv("DB_DIALECT", "cassandra")

	expErr := invalidDialect{}

	_, err := NewORMFromEnv()

	assert.Equalf(t, expErr, err, "TestCaseFailed:Expected %v Got %v", expErr, err, "invalid dialect")
}

func TestNewORMFromEnv_Success(t *testing.T) {
	t.Skip("skipping tests in short mode ")

	t.Setenv("DB_SSL", "require")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_NAME", "mysql")
	t.Setenv("DB_PORT", "2001")
	t.Setenv("DB_DIALECT", mySQL)
	t.Setenv("DB_CERTIFICATE_FILE", "../../.github/setups/certFiles/client-cert.pem")
	t.Setenv("DB_KEY_FILE", "../../.github/setups/certFiles/client-key.pem")
	t.Setenv("DB_CA_CERTIFICATE_FILE", "../../.github/setups/certFiles/ca-cert.pem")

	_, err := NewORMFromEnv()

	assert.Nil(t, err, "TestCase Failed:Success Case")
}

func Test_createSSLConfig(t *testing.T) {
	caCertPath := "../../.github/setups/certFiles/ca-cert.pem"
	clientCertPath := "../../.github/setups/certFiles/client-cert.pem"
	clientKeyPath := "../../.github/setups/certFiles/client-key.pem"

	sslConfig, err := createSSLConfig(caCertPath, clientCertPath, clientKeyPath)

	assert.NotEmptyf(t, sslConfig.Certificates, "Test [%d] Failed: %v")
	assert.Nilf(t, err, "Test [%d] Failed: %v")
}

func Test_createSSLConfig_Failure(t *testing.T) {
	testCases := []struct {
		desc           string
		caCertPath     string
		clientCertPath string
		clientKeyPath  string
		expErr         error
	}{
		{"when ca certificate is invalid", "../.github/setups/certFiles/ca-cert.pem",
			"../../.github/setups/certFiles/client-cert.pem",
			"../../.github/setups/certFiles/client-key.pem",
			nil},
		{"when client certificate is invalid", "../../.github/setups/certFiles/ca-cert.pem",
			"../.github/setups/certFiles/client-cert.pem",
			"../../.github/setups/certFiles/client-key.pem",
			nil},
		{"when client private key file is invalid", "../../.github/setups/certFiles/ca-cert.pem",
			"../../.github/setups/certFiles/client-cert.pem",
			"../.github/setups/certFiles/client-key.pem",
			nil},
	}

	for i, tc := range testCases {
		sslConfig, err := createSSLConfig(tc.caCertPath, tc.clientCertPath, tc.clientKeyPath)

		assert.Emptyf(t, sslConfig.Certificates, "Test [%d] Failed: %v", i+1, tc.desc)
		assert.NotNilf(t, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}
