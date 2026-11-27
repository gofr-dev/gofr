package container

import (
	"fmt"
	"strconv"
	"strings"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
)

// DB_AUTH values. With dbAuthIAM a connector module supplies the connection through
// App.AddSQLDB, so the container does not open one from the DB_* config.
const (
	dbAuthPassword = "password"
	dbAuthIAM      = "iam"
	dialectSQLite  = "sqlite"
)

var (
	errUnsupportedDBAuth = fmt.Errorf("unsupported DB_AUTH, so SQL is not initialized; supported values are - %s, %s",
		dbAuthPassword, dbAuthIAM)
	errConflictingDBAuth = fmt.Errorf("DB_AUTH=%s conflicts with DB_IAM_AUTH=true, so SQL is not initialized; "+
		"set DB_AUTH=%s or remove DB_IAM_AUTH", dbAuthPassword, dbAuthIAM)
	errIAMAuthSQLite = fmt.Errorf("DB_AUTH=%s is not supported for %s, so SQL is not initialized", dbAuthIAM, dialectSQLite)
)

// createSQL opens the SQL connection described by the DB_* config, unless DB_AUTH says a
// connector module supplies it through App.AddSQLDB. The check lives here, not in
// sql.NewSQL, because dbresolver also calls NewSQL for each replica with the app's config:
// a replica with its own user and password must still open when the primary uses IAM.
func (c *Container) createSQL(conf config.Config) {
	if !opensSQLFromConfig(conf, c.Logger) {
		return
	}

	// Assign only a non-nil *sql.DB: storing a nil pointer in the DB interface makes c.SQL == nil false.
	if db := sql.NewSQL(conf, c.Logger, c.metricsManager); db != nil {
		c.SQL = db
	}
}

// opensSQLFromConfig reports whether the container should open a connection from the
// DB_* config. Without DB_DIALECT, NewSQL decides and logs as before. With DB_AUTH=iam a
// connection opened here could never authenticate, so none is opened. An invalid or
// conflicting DB_AUTH is logged and nothing is opened, so a typo never silently falls
// back to password authentication.
func opensSQLFromConfig(conf config.Config, logger logging.Logger) bool {
	dialect := strings.TrimSpace(conf.Get("DB_DIALECT"))
	if dialect == "" {
		return true
	}

	auth, err := dbAuth(conf)
	if err != nil {
		logger.Errorf("%v", err)

		return false
	}

	if auth == dbAuthPassword {
		return true
	}

	if strings.EqualFold(dialect, dialectSQLite) {
		logger.Errorf("%v", errIAMAuthSQLite)

		return false
	}

	logger.Infof("DB_AUTH=%s or DB_IAM_AUTH=true is set, so the SQL connection is not opened from the DB_* config; "+
		"it is supplied by app.AddSQLDB", dbAuthIAM)

	return false
}

// dbAuth resolves DB_AUTH, defaulting to password. DB_IAM_AUTH=true, parsed as the
// cloudsql module parses it, also selects iam: the cloudsql module reads only
// DB_IAM_AUTH, so it is accepted without a deprecation warning until cloudsql reads DB_AUTH.
func dbAuth(conf config.Config) (string, error) {
	raw := strings.TrimSpace(conf.Get("DB_AUTH"))
	auth := strings.ToLower(raw)
	legacyIAM, _ := strconv.ParseBool(strings.TrimSpace(conf.Get("DB_IAM_AUTH")))

	switch {
	case auth != "" && auth != dbAuthPassword && auth != dbAuthIAM:
		return "", fmt.Errorf("%w: %q", errUnsupportedDBAuth, raw)
	case legacyIAM && auth == dbAuthPassword:
		return "", errConflictingDBAuth
	case legacyIAM || auth == dbAuthIAM:
		return dbAuthIAM, nil
	default:
		return dbAuthPassword, nil
	}
}
