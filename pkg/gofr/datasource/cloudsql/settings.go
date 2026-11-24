package cloudsql

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"cloud.google.com/go/cloudsqlconn"
)

const (
	ipTypePublic  = "PUBLIC"
	ipTypePrivate = "PRIVATE"
	ipTypePSC     = "PSC"

	dialectPostgres = "postgres"
	dialectMySQL    = "mysql"

	// Accepted input aliases that normalize to dialectPostgres.
	aliasPostgreSQL = "postgresql"
	aliasPgx        = "pgx"
)

// settings holds the Cloud SQL connector parameters parsed from configuration.
// It is only used on the IAM path; the non-IAM path keeps GoFr's standard SQL
// datasource, which reads its own configuration. Pool sizing is intentionally not
// here — GoFr applies DB_MAX_*_CONNECTION itself when it wraps the connection, so
// both paths size the pool identically.
type settings struct {
	instanceConnectionName string // DB_HOST, "project:region:instance"
	dialect                string // normalized: postgres | mysql
	database               string // DB_NAME
	user                   string // DB_USER (IAM principal)
	ipType                 string // normalized: PUBLIC | PRIVATE | PSC
}

func parseSettings(conf Config) settings {
	return settings{
		instanceConnectionName: conf.Get("DB_HOST"),
		dialect:                normalizeDialect(conf.Get("DB_DIALECT")),
		database:               conf.Get("DB_NAME"),
		user:                   conf.Get("DB_USER"),
		ipType:                 normalizeIPType(conf.GetOrDefault("DB_CLOUDSQL_IP_TYPE", ipTypePublic)),
	}
}

// iamRequested reports whether IAM database authentication was asked for. It is
// the single switch that decides between the Cloud SQL connector (IAM) and the
// standard SQL datasource (username/password) — so the same code and the same
// AddSQLDB call work locally and on GCP, with only configuration changing.
func iamRequested(conf Config) bool {
	// ParseBool accepts the conventional spellings (1/t/T/true/TRUE), matching how
	// the rest of GoFr reads boolean config; TrimSpace tolerates padded values and
	// an unset/invalid value parses as false.
	enabled, _ := strconv.ParseBool(strings.TrimSpace(conf.Get("DB_IAM_AUTH")))
	return enabled
}

// connectorOptions assembles the Cloud SQL connector options: IAM auth plus the
// IP connectivity type.
//
// Built-in metrics are turned off explicitly. cloudsqlconn v1.26.0 made them default-on, which
// would make every cloudsql app export to Cloud Monitoring and need roles/monitoring.metricWriter.
// GoFr keeps the prior behavior here; a dependency bump must not add telemetry or an IAM
// requirement on its own. A future opt-in can expose this as a setting.
func (s *settings) connectorOptions() []cloudsqlconn.Option {
	return []cloudsqlconn.Option{
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(s.ipDialOption()),
		cloudsqlconn.WithBuiltInMetrics(false),
	}
}

func (s *settings) ipDialOption() cloudsqlconn.DialOption {
	switch s.ipType {
	case ipTypePrivate:
		return cloudsqlconn.WithPrivateIP()
	case ipTypePSC:
		return cloudsqlconn.WithPSC()
	default:
		return cloudsqlconn.WithPublicIP()
	}
}

// postgresDSN builds the pgx connection string for IAM auth. TLS and credentials are
// handled by the connector at the dialer layer, so no password is set and sslmode is
// disabled on the driver itself. Values are quoted per libpq rules so a value
// containing a space or quote can't break out and inject another keyword.
func (s *settings) postgresDSN() string {
	return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
		quotePGValue(s.instanceConnectionName), quotePGValue(s.user), quotePGValue(s.database))
}

// quotePGValue renders a libpq connection-string value. Values without a unicode
// space, single quote or backslash are returned as-is (keeping the common case
// readable); otherwise the value is single-quoted with backslash escaping, so it
// stays a single token and can't inject further keywords. A value libpq already
// reads back as written is left alone, so .env files quoted by hand keep connecting.
// An empty value is quoted too, matching the copy in datasource/sql, which cannot
// be shared: this module does not import gofr.dev.
func quotePGValue(v string) string {
	if isQuotedPGValue(v) || isEscapedPGValue(v) {
		return v
	}

	if v != "" && !strings.ContainsAny(v, "'\\") && !strings.ContainsFunc(v, unicode.IsSpace) {
		return v
	}

	var b strings.Builder

	b.WriteByte('\'')

	for _, r := range v {
		if r == '\\' || r == '\'' {
			b.WriteByte('\\')
		}

		b.WriteRune(r)
	}

	b.WriteByte('\'')

	return b.String()
}

// isQuotedPGValue reports whether v is already one complete single quoted libpq token:
// wrapped in single quotes, with every quote and backslash inside escaped. libpq reads
// such a token back as the value it was, so quoting it again would add the quotes.
func isQuotedPGValue(v string) bool {
	if len(v) < 2 || v[0] != '\'' || v[len(v)-1] != '\'' {
		return false
	}

	escaped := false

	for _, r := range v[1 : len(v)-1] {
		if escaped {
			escaped = false

			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '\'':
			return false
		}
	}

	return !escaped
}

// isEscapedPGValue reports whether v already escapes a space, a quote or a backslash by hand,
// the way `p@ss\ w0rd`, `it\'s` and `ab\\cd` were written before this fix. A bare `ab\cd` has no
// pair, so it is still quoted, and a value starting with a quote is left to isQuotedPGValue.
func isEscapedPGValue(v string) bool {
	if strings.HasPrefix(v, "'") {
		return false
	}

	escaped, skipping := false, false

	for _, r := range v {
		if skipping {
			skipping = false

			if r == '\\' || r == '\'' || unicode.IsSpace(r) {
				escaped = true
			}

			continue
		}

		switch {
		case r == '\\':
			skipping = true
		case unicode.IsSpace(r):
			return false
		}
	}

	return escaped && !skipping
}

func normalizeDialect(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case dialectPostgres, aliasPostgreSQL, aliasPgx:
		return dialectPostgres
	case dialectMySQL:
		return dialectMySQL
	default:
		return ""
	}
}

func normalizeIPType(ipType string) string {
	switch strings.ToUpper(strings.TrimSpace(ipType)) {
	case ipTypePrivate:
		return ipTypePrivate
	case ipTypePSC:
		return ipTypePSC
	default:
		return ipTypePublic
	}
}
