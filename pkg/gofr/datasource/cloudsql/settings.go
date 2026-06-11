package cloudsql

import (
	"fmt"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"gofr.dev/pkg/gofr/config"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

const (
	ipTypePublic  = "PUBLIC"
	ipTypePrivate = "PRIVATE"
	ipTypePSC     = "PSC"

	dialectPostgres = "postgres"
	dialectMySQL    = "mysql"

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
	return strings.EqualFold(strings.TrimSpace(conf.Get("DB_IAM_AUTH")), "true")
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
// on the driver itself.
func (s *settings) postgresDSN() string {
	return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
		s.instanceConnectionName, s.user, s.database)
}

// mysqlDSN builds the go-sql-driver DSN for IAM auth. net must match the registered
// connector driver name; no password is set for IAM auth.
func (s *settings) mysqlDSN(net string) string {
	return fmt.Sprintf("%s@%s(%s)/%s?parseTime=true",
		s.user, net, s.instanceConnectionName, s.database)
}

func normalizeDialect(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "postgres", "postgresql", "pgx":
		return dialectPostgres
	case "mysql":
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
	n := 0
	// Ignore parse errors: an unset or invalid value falls back to the standard
	// SQL datasource defaults, matching getDBConfig in the core SQL package.
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &n)

	return n
}
