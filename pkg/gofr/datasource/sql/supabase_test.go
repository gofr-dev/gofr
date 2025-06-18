package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestGetSupabaseConfig(t *testing.T) {
	testCases := []struct {
		name     string
		configs  map[string]string
		expected *SupabaseConfig
	}{
		{
			name: "Complete Supabase Config",
			configs: map[string]string{
				"DB_DIALECT":               "supabase",
				"DB_HOST":                  "db.xyz.supabase.co",
				"DB_USER":                  "postgres",
				"DB_PASSWORD":              "password",
				"DB_PORT":                  "5432",
				"DB_NAME":                  "postgres",
				"DB_SSL_MODE":              "require",
				"SUPABASE_PROJECT_REF":     "xyz",
				"SUPABASE_CONNECTION_TYPE": "direct",
				"SUPABASE_REGION":          "us-east-1",
			},
			expected: &SupabaseConfig{
				DBConfig: &DBConfig{
					Dialect:     "supabase",
					HostName:    "db.xyz.supabase.co",
					User:        "postgres",
					Password:    "password",
					Port:        "5432",
					Database:    "postgres",
					SSLMode:     "require",
					MaxIdleConn: 2, // default value
					MaxOpenConn: 0, // default value
				},
				ConnectionType: "direct",
				ProjectRef:     "xyz",
				Region:         "us-east-1",
			},
		},
		{
			name: "Non-Supabase Dialect",
			configs: map[string]string{
				"DB_DIALECT": "postgres",
				"DB_HOST":    "localhost",
			},
			expected: nil,
		},
		{
			name: "With DB_URL",
			configs: map[string]string{
				"DB_DIALECT":  "supabase",
				"DB_PASSWORD": "password",
				"DB_SSL_MODE": "disable", // should be overridden to require
				"DB_URL":      "postgresql://postgres:password@db.xyz123.supabase.co:5432/postgres",
			},
			expected: &SupabaseConfig{
				DBConfig: &DBConfig{
					Dialect:     "supabase",
					Password:    "password",
					SSLMode:     "require", // should be overridden
					MaxIdleConn: 2,         // default value
					MaxOpenConn: 0,         // default value
				},
				ConnectionType: "direct", // default
				ProjectRef:     "xyz123",
				Region:         "",
			},
		},
		{
			name: "With Default Values",
			configs: map[string]string{
				"DB_DIALECT":           "supabase",
				"SUPABASE_PROJECT_REF": "abc",
			},
			expected: &SupabaseConfig{
				DBConfig: &DBConfig{
					Dialect:     "supabase",
					SSLMode:     "require",
					MaxIdleConn: 2, // default value
					MaxOpenConn: 0, // default value
				},
				ConnectionType: "direct", // default
				ProjectRef:     "abc",
				Region:         "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(tc.configs)
			result := GetSupabaseConfig(mockConfig)

			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected.DBConfig.Dialect, result.DBConfig.Dialect)
				assert.Equal(t, tc.expected.DBConfig.SSLMode, result.DBConfig.SSLMode)
				assert.Equal(t, tc.expected.ConnectionType, result.ConnectionType)
				assert.Equal(t, tc.expected.ProjectRef, result.ProjectRef)
				assert.Equal(t, tc.expected.Region, result.Region)
			}
		})
	}
}

func TestConfigureSupabaseConnection(t *testing.T) {
	testCases := getSupabaseConnectionTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs := testutil.StdoutOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)

				configureSupabaseConnection(tc.config, mockLogger)
			})

			assertSupabaseConnectionConfig(t, &tc, logs)
		})
	}
}

func getSupabaseConnectionTestCases() []struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return []struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		getDirectConnectionCase(),
		getSessionPoolerCase(),
		getTransactionPoolerCase(),
		getUnknownConnectionCase(),
		getEmptyDBNameCase(),
		getNonRequireSSLCase(),
	}
}

func getDirectConnectionCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Direct Connection",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "require",
			},
			ConnectionType: "direct",
			ProjectRef:     "xyz",
		},
		expectedHost:    "db.xyz.supabase.co",
		expectedPort:    "5432",
		expectedUser:    "postgres",
		expectedSSLMode: "require",
		logContains:     "Configured direct connection to Supabase",
	}
}

func getSessionPoolerCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Session Pooler Connection",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "require",
			},
			ConnectionType: "session",
			ProjectRef:     "xyz",
			Region:         "us-east-1",
		},
		expectedHost:    "aws-0-us-east-1.pooler.supabase.co",
		expectedPort:    "5432",
		expectedUser:    "postgres.xyz",
		expectedSSLMode: "require",
		logContains:     "Configured session pooler connection to Supabase",
	}
}

func getTransactionPoolerCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Transaction Pooler Connection",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "require",
			},
			ConnectionType: "transaction",
			ProjectRef:     "xyz",
			Region:         "us-east-1",
		},
		expectedHost:    "aws-0-us-east-1.pooler.supabase.co",
		expectedPort:    "6543",
		expectedUser:    "postgres.xyz",
		expectedSSLMode: "require",
		logContains:     "Configured transaction pooler connection to Supabase",
	}
}

func getUnknownConnectionCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Unknown Connection Type",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "require",
			},
			ConnectionType: "unknown",
			ProjectRef:     "xyz",
		},
		expectedHost:    "db.xyz.supabase.co",
		expectedPort:    "5432",
		expectedUser:    "postgres",
		expectedSSLMode: "require",
		logContains:     "Unknown Supabase connection type 'unknown', defaulting to direct connection",
	}
}

func getEmptyDBNameCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Default Database For Empty Database Name",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "require",
				Database: "",
			},
			ConnectionType: "direct",
			ProjectRef:     "xyz",
		},
		expectedHost:    "db.xyz.supabase.co",
		expectedPort:    "5432",
		expectedUser:    "postgres",
		expectedSSLMode: "require",
		logContains:     "Configured direct connection to Supabase",
	}
}

func getNonRequireSSLCase() struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
} {
	return struct {
		name            string
		config          *SupabaseConfig
		expectedHost    string
		expectedPort    string
		expectedUser    string
		expectedSSLMode string
		logContains     string
	}{
		name: "Direct Connection With Non-Require SSL Mode",
		config: &SupabaseConfig{
			DBConfig: &DBConfig{
				Dialect:  "supabase",
				User:     "postgres",
				Password: "password",
				SSLMode:  "disable",
			},
			ConnectionType: "direct",
			ProjectRef:     "xyz",
		},
		expectedHost:    "db.xyz.supabase.co",
		expectedPort:    "5432",
		expectedUser:    "postgres",
		expectedSSLMode: "require", // Should be forced to require
		logContains:     "Supabase connections require SSL. Setting DB_SSL_MODE to 'require'",
	}
}

func assertSupabaseConnectionConfig(t *testing.T, tc *struct {
	name            string
	config          *SupabaseConfig
	expectedHost    string
	expectedPort    string
	expectedUser    string
	expectedSSLMode string
	logContains     string
}, logs string) {
	t.Helper()
	assert.Equal(t, tc.expectedHost, tc.config.DBConfig.HostName)
	assert.Equal(t, tc.expectedPort, tc.config.DBConfig.Port)
	assert.Equal(t, tc.expectedSSLMode, tc.config.DBConfig.SSLMode)

	if tc.expectedUser != tc.config.DBConfig.User {
		assert.Equal(t, tc.expectedUser, tc.config.DBConfig.User)
	}

	assert.Contains(t, logs, tc.logContains)
}

func TestExtractProjectRefFromConnStr(t *testing.T) {
	testCases := []struct {
		name        string
		connStr     string
		expectedRef string
	}{
		{
			name:        "Valid Supabase Connection String",
			connStr:     "postgresql://postgres:password@db.abc123.supabase.co:5432/postgres",
			expectedRef: "abc123",
		},
		{
			name:        "Valid Connection String With Extra Parts",
			connStr:     "postgresql://postgres:password@db.xyz789.supabase.co:5432/postgres?sslmode=require",
			expectedRef: "xyz789",
		},
		{
			name:        "Invalid Format - No @ Symbol",
			connStr:     "postgresql://postgres:password-db.abc123.supabase.co:5432/postgres",
			expectedRef: "",
		},
		{
			name:        "Invalid Format - No db. Prefix",
			connStr:     "postgresql://postgres:password@wrongprefix.abc123.supabase.co:5432/postgres",
			expectedRef: "",
		},
		{
			name:        "Invalid Format - Not Supabase Domain",
			connStr:     "postgresql://postgres:password@db.abc123.postgresql.com:5432/postgres",
			expectedRef: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractProjectRefFromConnStr(tc.connStr)
			assert.Equal(t, tc.expectedRef, result)
		})
	}
}

func TestNewSupabaseSQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name        string
		configs     map[string]string
		expectNil   bool
		logContains string
	}{
		{
			name: "Valid Supabase Config",
			configs: map[string]string{
				"DB_DIALECT":               "supabase",
				"DB_USER":                  "postgres",
				"DB_PASSWORD":              "password",
				"DB_SSL_MODE":              "require",
				"SUPABASE_PROJECT_REF":     "abc123",
				"SUPABASE_CONNECTION_TYPE": "direct",
			},
			expectNil:   false,
			logContains: "Configured direct connection to Supabase",
		},
		{
			name: "Non-Supabase Dialect",
			configs: map[string]string{
				"DB_DIALECT": "postgres",
				"DB_HOST":    "localhost",
			},
			expectNil:   true,
			logContains: "",
		},
		{
			name: "Empty DB_HOST with automatic configuration",
			configs: map[string]string{
				"DB_DIALECT":               "supabase",
				"DB_USER":                  "postgres",
				"DB_PASSWORD":              "password",
				"SUPABASE_PROJECT_REF":     "abc123",
				"SUPABASE_CONNECTION_TYPE": "direct",
			},
			expectNil:   false,
			logContains: "connecting to 'postgres' user to 'postgres' database at 'db.abc123.supabase.co:5432'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(tc.configs)
			mockMetrics := NewMockMetrics(ctrl)

			// We expect metrics to be set regardless of the result
			mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

			logs := testutil.StdoutOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				result := NewSupabaseSQL(mockConfig, mockLogger, mockMetrics)

				if tc.expectNil {
					assert.Nil(t, result)
				} else {
					assert.NotNil(t, result)
				}
			})

			if tc.logContains != "" {
				assert.Contains(t, logs, tc.logContains)
			}
		})
	}
}

func TestIsSupabaseDialect(t *testing.T) {
	assert.True(t, IsSupabaseDialect("supabase"))
	assert.False(t, IsSupabaseDialect("postgres"))
	assert.False(t, IsSupabaseDialect("mysql"))
	assert.False(t, IsSupabaseDialect(""))
}

func TestSupabaseWithConnectionString(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test that full connection strings are preserved
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_DIALECT":           "supabase",
		"DB_USER":              "postgresql://postgres:password@db.abc123.supabase.co:5432/postgres",
		"DB_URL":               "postgresql://postgres:password@db.xyz789.supabase.co:5432/postgres",
		"SUPABASE_PROJECT_REF": "should-be-ignored", // Should extract from connection string instead
	})

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	// We expect metrics to be set
	mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

	supaConfig := GetSupabaseConfig(mockConfig)
	assert.NotNil(t, supaConfig)

	// When DB_USER contains a full connection string, it should be preserved
	assert.Equal(t, "postgresql://postgres:password@db.abc123.supabase.co:5432/postgres", supaConfig.DBConfig.User)

	// Project ref should be extracted from the connection string
	assert.Equal(t, "xyz789", supaConfig.ProjectRef)

	logs := testutil.StdoutOutputForFunc(func() {
		configureSupabaseConnection(supaConfig, mockLogger)
	})

	// Should not modify the connection string
	assert.Equal(t, "postgresql://postgres:password@db.abc123.supabase.co:5432/postgres", supaConfig.DBConfig.User)

	// No host/port configuration should happen
	assert.NotContains(t, logs, "Configured direct connection to Supabase")
}
