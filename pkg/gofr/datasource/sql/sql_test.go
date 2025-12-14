package sql

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewSQL_ErrorCase(t *testing.T) {
	ctrl := gomock.NewController(t)

	expectedLog := "could not connect 'testuser' user to 'testdb' database at 'localhost:3306'"

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "mysql",
		"DB_HOST":     "localhost",
		"DB_USER":     "testuser",
		"DB_PASSWORD": "testpassword",
		"DB_PORT":     "3306",
		"DB_NAME":     "testdb",
		"DB_SSL_MODE": "disable",
	})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.ERROR)
		mockMetrics := NewMockMetrics(ctrl)

		mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

		db := NewSQL(mockConfig, mockLogger, mockMetrics)

		// Fix: Stop the goroutine by closing the DB connection
		if db != nil && db.DB != nil {
			db.Close()
		}

		time.Sleep(10 * time.Millisecond)
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
		"DB_CHARSET":             "utf8mb4",
	})

	expectedConfigs := &DBConfig{
		Dialect:     "mysql",
		HostName:    "host",
		User:        "user",
		Password:    "password",
		Port:        "3201",
		Database:    "test",
		SSLMode:     "require",
		MaxIdleConn: 25,
		MaxOpenConn: 50,
		Charset:     "utf8mb4",
	}

	configs := getDBConfig(mockConfig)

	assert.Equal(t, expectedConfigs, configs)
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
			desc: "mysql dialect with Configurable charset",
			configs: &DBConfig{
				Dialect:  "mysql",
				HostName: "host",
				User:     "user",
				Password: "password",
				Port:     "3201",
				Database: "test",
				Charset:  "utf8mb4",
			},
			expOut: "user:password@tcp(host:3201)/test?charset=utf8mb4&parseTime=True&loc=Local&interpolateParams=true",
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
			desc: "cockroachdb dialect",
			configs: &DBConfig{
				Dialect:  "cockroachdb",
				HostName: "host",
				User:     "user",
				Password: "password",
				Port:     "26257",
				Database: "test",
				SSLMode:  "require",
			},
			expOut: "host=host port=26257 user=user password=password dbname=test sslmode=require",
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

func TestNewSQL_CockroachDB(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT":  "cockroachdb",
		"DB_HOST":     "localhost",
		"DB_USER":     "testuser",
		"DB_PASSWORD": "testpassword",
		"DB_PORT":     "26257",
		"DB_NAME":     "testdb",
		"DB_SSL_MODE": "disable",
	})

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

	testLogs := testutil.StderrOutputForFunc(func() {
		db := NewSQL(mockConfig, mockLogger, mockMetrics)
		assert.NotNil(t, db, "Expected a non-nil DB object for cockroachdb")

		if db != nil {
			assert.Equal(t, "cockroachdb", db.Dialect(), "Expected dialect to be cockroachdb")
		}
	})

	fmt.Println("Test Logs for CockroachDB:", testLogs)
}

func TestGetServerName(t *testing.T) {
	tests := []struct {
		hostname string
		expected string
	}{
		{"127.0.0.1", "localhost"},
		{"::1", "localhost"},
		{"db.example.com", "db.example.com"},
		{"192.168.1.100", "192.168.1.100"},
	}

	for _, tt := range tests {
		result := getServerName(tt.hostname)

		assert.Equal(t, tt.expected, result)
	}
}

func TestGetMySQLTLSParam(t *testing.T) {
	tests := []struct {
		sslMode  string
		expected string
	}{
		{"disable", ""},
		{"require", "tls=skip-verify"},
		{"verify-ca", "tls=custom"},
		{"verify-full", "tls=custom"},
		{"preferred", "tls=preferred"},
	}

	for _, tt := range tests {
		result := getMySQLTLSParam(tt.sslMode)

		assert.Equal(t, tt.expected, result)
	}
}

func TestRegisterMySQLTLSConfig_WithValidCA(t *testing.T) {
	tests := []struct {
		name       string
		setupCerts func(t *testing.T) map[string]string
		sslMode    string
		wantErr    bool
	}{
		{
			name: "MySQL verify-ca with valid CA cert",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				return map[string]string{"ca": caCertPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
		{
			name: "MySQL verify-full with valid CA cert",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				return map[string]string{"ca": caCertPath}
			},
			sslMode: "verify-full",
			wantErr: false,
		},
		{
			name: "MySQL with 127.0.0.1 hostname - uses localhost as ServerName",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				return map[string]string{"ca": caCertPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
		{
			name: "MySQL with ::1 hostname - uses localhost as ServerName",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				return map[string]string{"ca": caCertPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPaths := tt.setupCerts(t)
			defer cleanupCerts(certPaths)

			mockLogger := logging.NewMockLogger(logging.DEBUG)

			dbConfig := &DBConfig{
				Dialect:  "mysql",
				HostName: "localhost",
				SSLMode:  tt.sslMode,
			}

			err := registerMySQLTLSConfig(dbConfig, mockLogger)
			assert.NoError(t, err)
		})
	}
}

func TestRegisterMySQLTLSConfig_WithMutualTLS(t *testing.T) {
	tests := []struct {
		name       string
		setupCerts func(t *testing.T) map[string]string
		sslMode    string
		wantErr    bool
	}{
		{
			name: "MySQL with valid CA and client certificates",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				clientCertPath, clientKeyPath := createValidClientCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_CERT", clientCertPath)
				t.Setenv("DB_TLS_CLIENT_KEY", clientKeyPath)
				return map[string]string{"ca": caCertPath, "cert": clientCertPath, "key": clientKeyPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
		{
			name: "MySQL verify-full with mutual TLS",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				clientCertPath, clientKeyPath := createValidClientCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_CERT", clientCertPath)
				t.Setenv("DB_TLS_CLIENT_KEY", clientKeyPath)
				return map[string]string{"ca": caCertPath, "cert": clientCertPath, "key": clientKeyPath}
			},
			sslMode: "verify-full",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPaths := tt.setupCerts(t)
			defer cleanupCerts(certPaths)

			mockLogger := logging.NewMockLogger(logging.DEBUG)

			dbConfig := &DBConfig{
				Dialect:  "mysql",
				HostName: "localhost",
				SSLMode:  tt.sslMode,
			}

			err := registerMySQLTLSConfig(dbConfig, mockLogger)
			assert.NoError(t, err)
		})
	}
}

func TestRegisterMySQLTLSConfig_PartialClientCert(t *testing.T) {
	tests := []struct {
		name       string
		setupCerts func(t *testing.T) map[string]string
		sslMode    string
		wantErr    bool
	}{
		{
			name: "MySQL with only client cert - no key provided",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				clientCertPath, _ := createValidClientCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_CERT", clientCertPath)
				return map[string]string{"ca": caCertPath, "cert": clientCertPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
		{
			name: "MySQL with only client key - no cert provided",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				_, clientKeyPath := createValidClientCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_KEY", clientKeyPath)
				return map[string]string{"ca": caCertPath, "key": clientKeyPath}
			},
			sslMode: "verify-ca",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPaths := tt.setupCerts(t)
			defer cleanupCerts(certPaths)

			mockLogger := logging.NewMockLogger(logging.DEBUG)

			dbConfig := &DBConfig{
				Dialect:  "mysql",
				HostName: "localhost",
				SSLMode:  tt.sslMode,
			}

			err := registerMySQLTLSConfig(dbConfig, mockLogger)
			assert.NoError(t, err)
		})
	}
}

func TestRegisterMySQLTLSConfig_InvalidClientCert(t *testing.T) {
	tests := []struct {
		name       string
		setupCerts func(t *testing.T) map[string]string
		sslMode    string
		wantErr    bool
	}{
		{
			name: "MySQL with invalid client certificate file",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				invalidCertPath := createInvalidCert(t)
				_, clientKeyPath := createValidClientCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_CERT", invalidCertPath)
				t.Setenv("DB_TLS_CLIENT_KEY", clientKeyPath)
				return map[string]string{"ca": caCertPath, "cert": invalidCertPath, "key": clientKeyPath}
			},
			sslMode: "verify-ca",
			wantErr: true,
		},
		{
			name: "MySQL with invalid client key file",
			setupCerts: func(t *testing.T) map[string]string {
				t.Helper()
				caCertPath := createValidCACert(t)
				clientCertPath, _ := createValidClientCert(t)
				invalidKeyPath := createInvalidCert(t)
				t.Setenv("DB_TLS_CA_CERT", caCertPath)
				t.Setenv("DB_TLS_CLIENT_CERT", clientCertPath)
				t.Setenv("DB_TLS_CLIENT_KEY", invalidKeyPath)
				return map[string]string{"ca": caCertPath, "cert": clientCertPath, "key": invalidKeyPath}
			},
			sslMode: "verify-ca",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPaths := tt.setupCerts(t)
			defer cleanupCerts(certPaths)

			mockLogger := logging.NewMockLogger(logging.DEBUG)

			dbConfig := &DBConfig{
				Dialect:  "mysql",
				HostName: "localhost",
				SSLMode:  tt.sslMode,
			}

			err := registerMySQLTLSConfig(dbConfig, mockLogger)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to load client certificate")
		})
	}
}

func createValidCACert(t *testing.T) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-ca-*.pem")
	require.NoError(t, err)

	caCert := `-----BEGIN CERTIFICATE-----
MIIDvTCCAqWgAwIBAgIUJz8pRPZUcAMOWjDIFs+5pA88kdswDQYJKoZIhvcNAQEL
BQAwZjELMAkGA1UEBhMCVVMxDjAMBgNVBAgMBVN0YXRlMQ0wCwYDVQQHDARDaXR5
MRUwEwYDVQQKDAxPcmdhbml6YXRpb24xDTALBgNVBAsMBFVuaXQxEjAQBgNVBAMM
CWxvY2FsaG9zdDAeFw0yNTEyMDgxMDE1MTZaFw0zNTEyMDYxMDE1MTZaMGYxCzAJ
BgNVBAYTAlVTMQ4wDAYDVQQIDAVTdGF0ZTENMAsGA1UEBwwEQ2l0eTEVMBMGA1UE
CgwMT3JnYW5pemF0aW9uMQ0wCwYDVQQLDARVbml0MRIwEAYDVQQDDAlsb2NhbGhv
c3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCgoYPEAOt59viipZCS
2ziFAcsCWKO2A7kLnTXkrwJYu0qkDf/liT1Eh+XYRDeURJd/9kjZoLpKKsY9ychI
Ap7beZK2YP5ZjqpP4xg4n66hdXlaUW4zW0PnjrlrG41yKx+t4U/pvw/C8it/x1eV
SLkPqb6cKB7Gibuu8CaqGTB6Dcn4OTM36jMqLvwniNoowU9TpUriONnnwKSevA/y
Q2PVcfF7dsgVxN7FkpGvB5YDhA3ZILIudBEDQsHzPeMWW/OzCCD50OEzvVTNbcxK
Jm8LpD43fWhnpZ/rd5t10/d2AESZTFP4IKMIxIY6ZZNtg7khlBEPClQtOfPnr3IN
w731AgMBAAGjYzBhMB0GA1UdDgQWBBQIV03Kq2NkXPSiizzpatoaYeLtPjAfBgNV
HSMEGDAWgBQIV03Kq2NkXPSiizzpatoaYeLtPjAPBgNVHRMBAf8EBTADAQH/MA4G
A1UdDwEB/wQEAwIBhjANBgkqhkiG9w0BAQsFAAOCAQEAAaf4ZKoeFnxtUBAD1WI+
bHYezP8kQ0qSpXd5685SQ4EfG7zrEjzXMM19JCemss3euiJ2AgoqCRRPAtPTc2IR
Y99NvmoNnIlISaG2pmI5M0I9YKNdD8D8y/Dm6DQoBJ7gSlAIzKlWTT+wmeJmGFBW
+N95qB2BqoOlXF707ngnEA26o0Phdwvl+H006CebAA1vx7ZTln5CCjEd6VWZ/8Jg
Q+JQBufVKWbvnEcERZXHPV8+hut4qLhJmKHW76/2da7wefsU2B2CuKVdfKHo9SSF
A3PeXPVPJSAoBmD0o7nmviZWP+TaIxBojSnDeE7eNSWNF2Ug/PJo/LHvyeaGuq+5
uw==
-----END CERTIFICATE-----`

	_, err = tmpFile.WriteString(caCert)
	require.NoError(t, err)

	tmpFile.Close()

	return tmpFile.Name()
}

func createValidClientCert(t *testing.T) (certName, keyName string) {
	t.Helper()

	certFile, err := os.CreateTemp(t.TempDir(), "test-client-cert-*.pem")
	require.NoError(t, err)

	clientCert := `-----BEGIN CERTIFICATE-----
MIIDhjCCAm6gAwIBAgIBAjANBgkqhkiG9w0BAQsFADBmMQswCQYDVQQGEwJVUzEO
MAwGA1UECAwFU3RhdGUxDTALBgNVBAcMBENpdHkxFTATBgNVBAoMDE9yZ2FuaXph
dGlvbjENMAsGA1UECwwEVW5pdDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI1MTIw
ODEwMTUxNloXDTM1MTIwNjEwMTUxNlowYzELMAkGA1UEBhMCVVMxDjAMBgNVBAgM
BVN0YXRlMQ0wCwYDVQQHDARDaXR5MRUwEwYDVQQKDAxPcmdhbml6YXRpb24xDTAL
BgNVBAsMBFVuaXQxDzANBgNVBAMMBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAPQxEO4bOB3U5qjGcl7D+qciCCOYvHnP5bTWjkI/0G+3LDa1
oJkhQsKn3jW7VlhUP9pnMiazS+Be2X4+NwcXX40jcBwPJTAFHdNnqtXwqaVeTzJv
nSRgBZvDNUEFjx14rHHqMBWqyaYPeLt2rd53dCxExHFtUJLyMiGKuv57YiNu00h7
umGTaOrZx2KjvssiDVo3MlckhKvr+H4MR6ashsP5Nx8bls3916iHX10APblpw6oZ
ZgrPY8Hw5ucL/dSyfAhlweUKwD/MT7P4OtXLtp6DX7UAdjC/YhXog4gIWurvCYAo
P7/2cl2+86JdjYXas55SfBoY9N9Y+rQQQO0I7SsCAwEAAaNCMEAwHQYDVR0OBBYE
FJWdyIL+p4qd4u7kGc1PsAa/tSZ5MB8GA1UdIwQYMBaAFAhXTcqrY2Rc9KKLPOlq
2hph4u0+MA0GCSqGSIb3DQEBCwUAA4IBAQAB70Pjep8SyWKJ2uqzHcMSO9VuNs37
BFRJ1F1zkmBmg5WmgJ61Cwf4PZofF5MRSQui0Bzkhi8A8pF558Sf8fZHkxQ0DHmH
jVNOp06K8BEfmpXVMR6AGwRx6WLjyoQ0g+z7xcIRhS3DDPz4R3WiTbOf4eZ0j+uq
GAYsLM9Ql4D6jdLPfn8A3mqy0xief9bj5dkLCkoEb4csPlmutrbSSTxEi+byEnQM
ASDTTY2dJpEJLXboZYwXY25R+69eVl6CjWnReGYrAoYueIS/1fJ0ik3wGvWZNtGz
TA8/qi+HLJqT79lYy5YVA0d4PuOJVyuAla4Mv64uyOv3g8HYvIUaWdMJ
-----END CERTIFICATE-----`

	_, err = certFile.WriteString(clientCert)
	require.NoError(t, err)

	certFile.Close()

	keyFile, err := os.CreateTemp(t.TempDir(), "test-client-key-*.pem")
	require.NoError(t, err)

	clientKey := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD0MRDuGzgd1Oao
xnJew/qnIggjmLx5z+W01o5CP9Bvtyw2taCZIULCp941u1ZYVD/aZzIms0vgXtl+
PjcHF1+NI3AcDyUwBR3TZ6rV8KmlXk8yb50kYAWbwzVBBY8deKxx6jAVqsmmD3i7
dq3ed3QsRMRxbVCS8jIhirr+e2IjbtNIe7phk2jq2cdio77LIg1aNzJXJISr6/h+
DEemrIbD+TcfG5bN/deoh19dAD25acOqGWYKz2PB8ObnC/3UsnwIZcHlCsA/zE+z
+DrVy7aeg1+1AHYwv2IV6IOICFrq7wmAKD+/9nJdvvOiXY2F2rOeUnwaGPTfWPq0
EEDtCO0rAgMBAAECggEAANICC2K7sLH3PRLpmHLnw9ROmwas1MCY4Molu92TWYS6
g8vevb+fBfYNaOMiZPU81QLVaCGLEYu6sadg2ke+/O46YVsVq2XLq1r6TRyxXTUG
EWvO5yvhaPFiG5VQB+/QrdNKamWNUYmqGgB0kL5C/Xu/qIfkUOdlDrgfbQfEv8y3
qph9IWUX35nUKDF5MzrT7nlafpHw64fXsxDrlwGZUJZr1tQdayMc0GJs6cnFalQH
VhZ1CfEebiWJ4JcYcrlS3MLP9jqiJsLdE6V9FNVHOF8JGyU2QHjlwRs8g/iAsNp9
BI3lCoNJrDNE4bvgSW8BxJMRaTjNjdjcBGtDpTFTwQKBgQD7LzkHs/jbgzkTETy/
z8V2PiAHSYSBtfkKOzeno8Uru4NdTZL0ruSdfNH9zvtfvEhUmqbGFUYIeLZLDetA
E15N1KzX7QMb3/X0D2L58QuE0TXzlnKDrYzfn5GF4b7Rl2zYxtooRTc1N5yokaiI
cbis2bj4zLod1FPq4enb0OEMQQKBgQD434YRk/TvbpAgPCCUWns3OA2D5iqVsdDx
d8pd0dk5GKiEEMNz5kf874xWpdW7kmp/AoKP/eFLFhqa7FhQqohnd1i6P223S3jA
NaheK3RcEZuMFBuJEaevOU5Se9NUM1MN/EPnVSgPCkurYHGOT6xaleSHCOgcokdN
gsFasf1OawKBgQCVj2KXsZNlsNaVAdh4JVBfvVH4xM9/JEjqzKOwz5ShG392WLA9
vL0nAKFQTKPkNwmiRosyuov+k1GHkvwWJPIryYw47UjCmjGqZlb6l4nSRXeoWFZL
DVUp+ar+WpHx3gXTdWOEQuJCb6B5xnDg/UWGtgSrL8tJ45kr6+QBHHhDgQKBgE86
2fO+pruS909L1RNlutRZg/P50pTVhy9Yc5RqujzzHLLuo0rChSiBGqx7HxAYDM9i
fS5aJN9CqjWoCHWl1Mcbt6OTjdpMrKSEcJWKQAEPmfV+cUWx2TBvjf+0bBLiRA6v
wO5krdwb6vskOQKVWsl77sUOkNaM0yZZ+jRldb8BAoGAMpVJkshl4tPEOSF4Df/V
m63wZdsQP/X6tqJj+spzcrE2+vr+dyZoBy/XsWsATTVctcXyFEmjvcDCQ1LO84Ax
WxwqJrZDjE25KEkBYof96+VCOeOijx/UO8IjYDSlW74MFFqHkpg/pbVLbHLcGkne
RNJCjPCWqmCTE2F26ABGVXM=
-----END PRIVATE KEY-----`

	_, err = keyFile.WriteString(clientKey)
	require.NoError(t, err)

	keyFile.Close()

	return certFile.Name(), keyFile.Name()
}

func createInvalidCert(t *testing.T) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-invalid-*.pem")
	require.NoError(t, err)

	_, err = tmpFile.WriteString("INVALID CERTIFICATE CONTENT")
	require.NoError(t, err)

	tmpFile.Close()

	return tmpFile.Name()
}

func cleanupCerts(certPaths map[string]string) {
	for _, path := range certPaths {
		os.Remove(path)
	}
}
