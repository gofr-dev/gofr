package container

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// The password cases open SQLite. The non-test driver registration is behind the
	// gofr_nosqldrivers build tag, so the test imports the driver itself and passes
	// in both builds, as the sql package's tests do.
	_ "modernc.org/sqlite"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/testutil"
)

func TestContainer_DBAuth(t *testing.T) {
	// The IAM host is an instance connection name: were a connection attempted, it would
	// log a "could not connect" error, which the iam cases assert never happens.
	iam := map[string]string{"DB_DIALECT": "postgres", "DB_HOST": "p:r:i", "DB_USER": "u", "DB_NAME": "d"}

	testCases := []struct {
		desc       string
		base       map[string]string
		auth       string
		legacyIAM  string
		wantOpened bool
		wantLog    string
	}{
		{desc: "unset defaults to password", wantOpened: true},
		{desc: "password opens the connection", auth: "password", wantOpened: true},
		{desc: "legacy flag false keeps password", legacyIAM: "false", wantOpened: true},
		{desc: "iam skips the connection", base: iam, auth: "iam", wantLog: "app.AddSQLDB"},
		{desc: "iam is case-insensitive", base: iam, auth: " IAM ", wantLog: "app.AddSQLDB"},
		{desc: "legacy flag means iam", base: iam, legacyIAM: "true", wantLog: "app.AddSQLDB"},
		{desc: "legacy flag accepts ParseBool spellings", base: iam, legacyIAM: "1", wantLog: "app.AddSQLDB"},
		{desc: "legacy flag agreeing with iam", base: iam, auth: "iam", legacyIAM: "true", wantLog: "app.AddSQLDB"},
		{desc: "legacy flag conflicting with password", base: iam, auth: "password", legacyIAM: "true",
			wantLog: errConflictingDBAuth.Error()},
		{desc: "unknown value names the value", base: iam, auth: "token", wantLog: `unsupported DB_AUTH`},
		{desc: "iam with sqlite", auth: "iam", wantLog: errIAMAuthSQLite.Error()},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			conf := tc.base
			if conf == nil {
				conf = map[string]string{"DB_DIALECT": dialectSQLite, "DB_NAME": filepath.Join(t.TempDir(), "t.db")}
			}

			conf = maps.Clone(conf)
			conf["DB_AUTH"] = tc.auth
			conf["DB_IAM_AUTH"] = tc.legacyIAM
			conf["LOG_LEVEL"] = "DEBUG"

			var c *Container

			logs := testutil.StderrOutputForFunc(func() {
				fmt.Fprint(os.Stderr, testutil.StdoutOutputForFunc(func() {
					c = NewContainer(config.NewMockConfig(conf))
				}))
			})

			if tc.wantOpened {
				require.NotNil(t, c.SQL)
				require.NoError(t, c.SQL.Close())
			} else {
				// Compare the interface itself: assert.Nil also passes for a typed nil.
				if c.SQL != nil {
					t.Errorf("expected untyped nil SQL, got %T", c.SQL)
				}

				assert.NotContains(t, logs, "could not connect")
			}

			if tc.wantLog != "" {
				assert.Contains(t, logs, tc.wantLog)
			}
		})
	}
}

func TestDBAuth_UnsupportedValueIsNamed(t *testing.T) {
	_, err := dbAuth(config.NewMockConfig(map[string]string{"DB_AUTH": " Token "}))

	require.ErrorIs(t, err, errUnsupportedDBAuth)
	assert.Contains(t, err.Error(), `"Token"`)
}

func TestDBAuth_ConflictMessage(t *testing.T) {
	_, err := dbAuth(config.NewMockConfig(map[string]string{"DB_AUTH": "password", "DB_IAM_AUTH": "true"}))

	require.ErrorIs(t, err, errConflictingDBAuth)
	assert.Equal(t, "DB_AUTH=password conflicts with DB_IAM_AUTH=true, so SQL is not initialized; "+
		"set DB_AUTH=iam or remove DB_IAM_AUTH", err.Error())
	assert.NotContains(t, err.Error(), "deprecated")
}
