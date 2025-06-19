package sql

import (
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	supabaseDialect          = "supabase"
	supabaseDirectHost       = "db.%s.supabase.co"
	supabasePoolerHost       = "aws-0-%s.pooler.supabase.co"
	directPort               = "5432"
	sessionPoolerPort        = "5432"
	transactionPoolerPort    = "6543"
	minConnectionStringParts = 2
)

// SupabaseConfig extends DBConfig to include Supabase-specific configuration.
type SupabaseConfig struct {
	*DBConfig
	ConnectionType string // direct, session, transaction
	ProjectRef     string // Supabase project reference
	Region         string // Supabase region
}

// GetSupabaseConfig builds a Supabase configuration from general config.
// It extracts Supabase-specific settings from the provided configs object.
// If the database dialect is not supabase, it returns nil.
func GetSupabaseConfig(configs config.Config) *SupabaseConfig {
	dbConfig := getDBConfig(configs)

	if dbConfig.Dialect != supabaseDialect {
		return nil
	}

	dbConfig.SSLMode = requireSSLMode // Enforce SSL mode for Supabase

	connectionType := strings.ToLower(configs.GetOrDefault("SUPABASE_CONNECTION_TYPE", "direct"))
	projectRef := configs.Get("SUPABASE_PROJECT_REF")
	region := configs.GetOrDefault("SUPABASE_REGION", "")

	// If a direct connection string is provided, we'll use that instead
	connStr := configs.Get("DB_URL")
	if connStr != "" {
		projectRef = extractProjectRefFromConnStr(connStr)
	}

	return &SupabaseConfig{
		DBConfig:       dbConfig,
		ConnectionType: connectionType,
		ProjectRef:     projectRef,
		Region:         region,
	}
}

// NewSupabaseSQL creates a new DB instance configured for Supabase connectivity.
// It initializes a DB with Supabase-specific settings based on the provided configs.
// If Supabase dialect is not specified, it returns nil.
func NewSupabaseSQL(configs config.Config, logger datasource.Logger, metrics Metrics) *DB {
	supaConfig := GetSupabaseConfig(configs)
	if supaConfig == nil {
		return nil
	}

	configureSupabaseConnection(supaConfig, logger)

	return NewSQL(configs, logger, metrics)
}

// configureSupabaseConnection sets up connection parameters based on the Supabase connection type.
// It configures the host, port, and user fields of the SupabaseConfig according to the
// connection type (direct, session, or transaction) and logs debug information.
func configureSupabaseConnection(supaConfig *SupabaseConfig, logger datasource.Logger) {
	connStr := supaConfig.User
	if strings.HasPrefix(connStr, "postgresql://") || strings.HasPrefix(connStr, "postgres://") {
		// User field might contain the full connection string
		// In this case, we'll keep it as is and return (NewSQL will handle it)
		return
	}

	if supaConfig.SSLMode != requireSSLMode {
		logger.Warnf("Supabase connections require SSL. Setting DB_SSL_MODE to 'require'")

		supaConfig.SSLMode = requireSSLMode
	}

	switch supaConfig.ConnectionType {
	case "direct":
		// Format: db.[PROJECT_REF].supabase.co
		supaConfig.HostName = fmt.Sprintf(supabaseDirectHost, supaConfig.ProjectRef)
		supaConfig.Port = directPort
		logger.Debugf("Configured direct connection to Supabase at %s:%s", supaConfig.HostName, supaConfig.Port)

	case "session":
		// Format: postgres.[PROJECT_REF]@aws-0-[REGION].pooler.supabase.co
		supaConfig.HostName = fmt.Sprintf(supabasePoolerHost, supaConfig.Region)
		supaConfig.User = fmt.Sprintf("postgres.%s", supaConfig.ProjectRef)
		supaConfig.Port = sessionPoolerPort
		logger.Debugf("Configured session pooler connection to Supabase at %s:%s", supaConfig.HostName, supaConfig.Port)

	case "transaction":
		// Format: postgres.[PROJECT_REF]@aws-0-[REGION].pooler.supabase.co
		supaConfig.HostName = fmt.Sprintf(supabasePoolerHost, supaConfig.Region)
		supaConfig.User = fmt.Sprintf("postgres.%s", supaConfig.ProjectRef)
		supaConfig.Port = transactionPoolerPort
		logger.Debugf("Configured transaction pooler connection to Supabase at %s:%s", supaConfig.HostName, supaConfig.Port)

	default:
		logger.Warnf("Unknown Supabase connection type '%s', defaulting to direct connection", supaConfig.ConnectionType)
		supaConfig.HostName = fmt.Sprintf(supabaseDirectHost, supaConfig.ProjectRef)
		supaConfig.Port = directPort
	}

	if supaConfig.Database == "" {
		supaConfig.Database = "postgres"
	}
}

// extractProjectRefFromConnStr extracts the Supabase project reference from a connection string.
// Connection string format is expected to be:
// postgresql://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
// It returns the project reference, or an empty string if it cannot be extracted.
func extractProjectRefFromConnStr(connStr string) string {
	// Expecting format like: postgresql://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
	parts := strings.Split(connStr, "@")

	if len(parts) < minConnectionStringParts {
		return ""
	}

	hostPart := strings.Split(parts[1], ":")[0]
	hostSegments := strings.Split(hostPart, ".")

	// Looking for the segment between "db." and ".supabase.co"
	if len(hostSegments) >= 3 && hostSegments[0] == "db" && strings.Contains(hostPart, "supabase.co") {
		return hostSegments[1]
	}

	return ""
}

// IsSupabaseDialect checks if the provided dialect is the Supabase dialect.
// Returns true if the dialect is "supabase", false otherwise.
func IsSupabaseDialect(dialect string) bool {
	return dialect == supabaseDialect
}
