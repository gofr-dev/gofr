package cloudsql

import (
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConfig is a minimal Config implementation for tests. It satisfies the local
// Config interface without importing GoFr — the whole point of this module is that
// it (and its tests) carry no gofr.dev dependency.
type fakeConfig map[string]string

func (f fakeConfig) Get(key string) string { return f[key] }

func (f fakeConfig) GetOrDefault(key, defaultValue string) string {
	if v, ok := f[key]; ok && v != "" {
		return v
	}

	return defaultValue
}

func TestNormalizeDialect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"postgres", "postgres", dialectPostgres},
		{"postgresql alias", "postgresql", dialectPostgres},
		{"pgx alias", "pgx", dialectPostgres},
		{"mixed case + spaces", "  PostGres  ", dialectPostgres},
		{"mysql", "mysql", dialectMySQL},
		{"mysql upper", "MYSQL", dialectMySQL},
		{"unsupported", "sqlite", ""},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeDialect(tc.input))
		})
	}
}

func TestNormalizeIPType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"private", "PRIVATE", ipTypePrivate},
		{"private lower", "private", ipTypePrivate},
		{"psc", "PSC", ipTypePSC},
		{"public", "PUBLIC", ipTypePublic},
		{"empty defaults to public", "", ipTypePublic},
		{"unknown defaults to public", "internal", ipTypePublic},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeIPType(tc.input))
		})
	}
}

func TestIAMRequested(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"true mixed case", "TRUE", true},
		{"true padded", "  true  ", true},
		{"numeric one", "1", true},
		{"single char t", "t", true},
		{"false", "false", false},
		{"numeric zero", "0", false},
		{"empty", "", false},
		{"garbage", "yes", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, iamRequested(fakeConfig{"DB_IAM_AUTH": tc.value}))
		})
	}
}

func TestParseSettings(t *testing.T) {
	s := parseSettings(fakeConfig{
		"DB_HOST":             "proj:us-central1:inst",
		"DB_DIALECT":          "postgresql", // alias normalizes to postgres
		"DB_NAME":             "app",
		"DB_USER":             "app-sa@proj.iam",
		"DB_CLOUDSQL_IP_TYPE": "private",
	})

	assert.Equal(t, "proj:us-central1:inst", s.instanceConnectionName)
	assert.Equal(t, dialectPostgres, s.dialect)
	assert.Equal(t, "app", s.database)
	assert.Equal(t, "app-sa@proj.iam", s.user)
	assert.Equal(t, ipTypePrivate, s.ipType)
}

func TestParseSettings_DefaultIPType(t *testing.T) {
	s := parseSettings(fakeConfig{"DB_DIALECT": "mysql"})
	assert.Equal(t, ipTypePublic, s.ipType, "ip type defaults to public when unset")
}

func TestSettings_PostgresDSN(t *testing.T) {
	s := settings{
		instanceConnectionName: "proj:us-central1:inst",
		user:                   "app-sa@proj.iam",
		database:               "app",
	}

	assert.Equal(t,
		"host=proj:us-central1:inst user=app-sa@proj.iam dbname=app sslmode=disable",
		s.postgresDSN())
}

// TestSettings_PostgresDSN_Escaping verifies values containing DSN-significant
// characters are quoted instead of breaking out into additional libpq keywords.
func TestSettings_PostgresDSN_Escaping(t *testing.T) {
	s := settings{
		instanceConnectionName: "proj:us-central1:inst",
		user:                   "app-sa@proj.iam",
		database:               "ap p' sslmode=require",
	}

	assert.Equal(t,
		`host=proj:us-central1:inst user=app-sa@proj.iam dbname='ap p\' sslmode=require' sslmode=disable`,
		s.postgresDSN())
}

func TestSettings_ConnectorOptions(t *testing.T) {
	// IAM auth plus exactly one IP-type dial option, for every IP type.
	for _, ip := range []string{ipTypePublic, ipTypePrivate, ipTypePSC, ""} {
		s := settings{ipType: normalizeIPType(ip)}
		assert.Len(t, s.connectorOptions(), 2, "ip=%q", ip)
	}
}

func TestNew(t *testing.T) {
	c := New(fakeConfig{"DB_IAM_AUTH": "false"})
	require.NotNil(t, c)
}

// TestConnector_Connect_Defers verifies that without IAM auth, Connect returns a nil
// connector (and nil cleanup/error), signaling App.AddSQLDB to keep GoFr's standard
// env-configured SQL connection.
func TestConnector_Connect_Defers(t *testing.T) {
	connector, cleanup, err := New(fakeConfig{"DB_IAM_AUTH": "false"}).Connect()

	require.NoError(t, err)
	assert.Nil(t, connector, "non-IAM Connect must not build a connector")
	assert.Nil(t, cleanup)
}

// TestConnector_Connect_IAMValidation verifies the IAM path validates configuration
// and fails (no connector) instead of attempting to dial, for the cases reachable
// without a live GCP instance.
func TestConnector_Connect_IAMValidation(t *testing.T) {
	tests := []struct {
		name    string
		configs fakeConfig
		wantErr error
	}{
		{
			name:    "unsupported dialect",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "mongo", "DB_HOST": "p:r:i"},
			wantErr: errUnsupportedDialect,
		},
		{
			name:    "missing instance connection name",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "postgres"},
			wantErr: errMissingInstance,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			connector, cleanup, err := New(tc.configs).Connect()

			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, connector)
			assert.Nil(t, cleanup)
		})
	}
}

// fakeADC is an authorized_user Application Default Credentials file. Loading it
// performs no network I/O, so the Cloud SQL dialer can be constructed offline; the
// connectors built from it are never dialed in these tests.
const fakeADC = `{"type":"authorized_user","client_id":"id","client_secret":"secret","refresh_token":"token"}`

// setFakeADC points Application Default Credentials at a temp file holding content.
func setFakeADC(t *testing.T, content string) {
	t.Helper()

	adcPath := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(adcPath, []byte(content), 0o600))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", adcPath)
}

// TestConnector_Connect_IAM verifies the IAM path builds a dialer-backed connector
// for each supported dialect and IP type without dialing, and that the returned
// cleanup tears the dialer down so the connector can no longer dial.
func TestConnector_Connect_IAM(t *testing.T) {
	tests := []struct {
		name    string
		configs fakeConfig
	}{
		{
			// An IP-literal host keeps pgx from doing a DNS lookup before it calls the
			// dial function, so the post-cleanup connect below stays offline.
			name:    "postgres public ip",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "postgres", "DB_HOST": "127.0.0.1", "DB_USER": "u", "DB_NAME": "d"},
		},
		{
			name: "mysql private ip",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "mysql", "DB_HOST": "p:r:i", "DB_USER": "u",
				"DB_NAME": "d", "DB_CLOUDSQL_IP_TYPE": "PRIVATE"},
		},
		{
			name:    "mysql psc",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "mysql", "DB_HOST": "p:r:i", "DB_CLOUDSQL_IP_TYPE": "PSC"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setFakeADC(t, fakeADC)

			connector, cleanup, err := New(tc.configs).Connect()

			require.NoError(t, err)
			assert.NotNil(t, connector)
			require.NotNil(t, cleanup)
			require.NoError(t, cleanup())

			// Once cleaned up, the connector's dial path routes through the closed
			// dialer, so connecting fails fast without any network I/O.
			_, err = connector.Connect(t.Context())
			require.ErrorIs(t, err, cloudsqlconn.ErrDialerClosed)
		})
	}
}

// TestConnector_Connect_IAMBuildError verifies failures while building the IAM
// connector (dialer credentials, postgres DSN parsing) are surfaced with no
// connector or cleanup.
func TestConnector_Connect_IAMBuildError(t *testing.T) {
	tests := []struct {
		name    string
		configs fakeConfig
		adc     string
		wantErr string
	}{
		{
			name:    "postgres dialer credentials invalid",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "postgres", "DB_HOST": "p:r:i"},
			adc:     "{not json",
			wantErr: "cloudsql: create dialer",
		},
		{
			name:    "mysql dialer credentials invalid",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "mysql", "DB_HOST": "p:r:i"},
			adc:     "{not json",
			wantErr: "cloudsql: create dialer",
		},
		{
			name:    "postgres config unparsable",
			configs: fakeConfig{"DB_IAM_AUTH": "true", "DB_DIALECT": "postgres", "DB_HOST": "p:r:i", "DB_NAME": "a\x00b"},
			adc:     fakeADC,
			wantErr: "cloudsql: parse postgres config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setFakeADC(t, tc.adc)

			connector, cleanup, err := New(tc.configs).Connect()

			require.ErrorContains(t, err, tc.wantErr)
			assert.Nil(t, connector)
			assert.Nil(t, cleanup)
		})
	}
}
