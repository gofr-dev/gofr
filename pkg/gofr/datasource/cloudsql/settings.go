package cloudsql

import (
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/go-sql-driver/mysql"
	"gofr.dev/pkg/gofr/config"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
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

	defaultMaxIdleConn = 2
)

// settings holds the Cloud SQL connector parameters parsed from configuration.
// It is only used on the IAM path; the non-IAM path defers entirely to the
// standard SQL datasource and reads configuration itself.
type settings struct {
	instanceConnectionName string // DB_HOST, "project:region:instance"
	dialect                string // normalized: postgres | mysql
	database               string // DB_NAME
	user                   string // DB_USER (IAM principal)
	ipType                 string // normalized: PUBLIC | PRIVATE | PSC
	maxIdleConn            int
	maxOpenConn            int
}

func parseSettings(conf config.Config) settings {
	return settings{
		instanceConnectionName: conf.Get("DB_HOST"),
		dialect:                normalizeDialect(conf.Get("DB_DIALECT")),
		database:               conf.Get("DB_NAME"),
		user:                   conf.Get("DB_USER"),
		ipType:                 normalizeIPType(conf.GetOrDefault("DB_CLOUDSQL_IP_TYPE", ipTypePublic)),
		maxIdleConn:            intOrZero(conf.Get("DB_MAX_IDLE_CONNECTION")),
		maxOpenConn:            intOrZero(conf.Get("DB_MAX_OPEN_CONNECTION")),
	}
}

// iamRequested reports whether IAM database authentication was asked for. It is
// the single switch that decides between the Cloud SQL connector (IAM) and the
// standard SQL datasource (username/password) — so the same code and the same
// AddSQLDB call work locally and on GCP, with only configuration changing.
func iamRequested(conf config.Config) bool {
	// ParseBool accepts the conventional spellings (1/t/T/true/TRUE), matching how
	// the rest of GoFr reads boolean config; TrimSpace tolerates padded values and
	// an unset/invalid value parses as false.
	enabled, _ := strconv.ParseBool(strings.TrimSpace(conf.Get("DB_IAM_AUTH")))
	return enabled
}

// connectorOptions assembles the Cloud SQL connector options: IAM auth plus the
// IP connectivity type.
func (s *settings) connectorOptions() []cloudsqlconn.Option {
	return []cloudsqlconn.Option{
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(s.ipDialOption()),
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

// dbConfig builds the gofrSQL.DBConfig used for logging, metrics labels, health
// output and pool sizing of the wrapped IAM connection.
func (s *settings) dbConfig() *gofrSQL.DBConfig {
	maxIdle := s.maxIdleConn
	if maxIdle == 0 {
		maxIdle = defaultMaxIdleConn
	}

	return &gofrSQL.DBConfig{
		Dialect:     s.dialect,
		HostName:    s.instanceConnectionName,
		Database:    s.database,
		User:        s.user,
		MaxIdleConn: maxIdle,
		MaxOpenConn: s.maxOpenConn,
	}
}

// postgresDSN builds the pgx DSN for IAM auth. TLS and credentials are handled by
// the connector at the dialer layer, so no password is set and sslmode is disabled
// on the driver itself. Values are quoted per libpq rules so a value containing a
// space or quote can't break out and inject another keyword.
func (s *settings) postgresDSN() string {
	return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
		quotePGValue(s.instanceConnectionName), quotePGValue(s.user), quotePGValue(s.database))
}

// mysqlDSN builds the go-sql-driver DSN for IAM auth via mysql.Config.FormatDSN,
// which escapes the user/database/address fields. net must match the registered
// connector driver name; no password is set for IAM auth.
func (s *settings) mysqlDSN(net string) string {
	// NewConfig seeds the driver's real defaults so FormatDSN omits them, keeping the
	// DSN to the fields we set (a zero Config would emit allowNativePasswords, etc.).
	cfg := mysql.NewConfig()
	cfg.User = s.user
	cfg.Net = net
	cfg.Addr = s.instanceConnectionName
	cfg.DBName = s.database
	cfg.ParseTime = true

	return cfg.FormatDSN()
}

// quotePGValue renders a libpq connection-string value. Values without a space,
// single quote or backslash are returned as-is (keeping the common case readable);
// otherwise the value is single-quoted with backslash escaping, so it stays a single
// token and can't inject further keywords.
func quotePGValue(v string) string {
	if v != "" && !strings.ContainsAny(v, " '\\") {
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

func intOrZero(s string) int {
	// Ignore parse errors: an unset or invalid value falls back to the standard
	// SQL datasource defaults, matching getDBConfig in the core SQL package. Atoi
	// (not Sscanf) is used so "5abc" is rejected rather than parsed as 5, keeping
	// this path strict like the core config reader.
	n, _ := strconv.Atoi(strings.TrimSpace(s))

	return n
}
