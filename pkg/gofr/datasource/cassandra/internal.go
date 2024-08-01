package cassandra

import (
	"regexp"
	"strings"

	"github.com/gocql/gocql"
)

// cassandraIterator implements iterator interface.
type cassandraIterator struct {
	iter *gocql.Iter
}

// Columns gets the column information.
// This method wraps the `Columns` method of the underlying `iter` object.
func (c *cassandraIterator) columns() []gocql.ColumnInfo {
	return c.iter.Columns()
}

// Scan gets the next row from the Cassandra iterator and fills in the provided arguments.
// This method wraps the `Scan` method of the underlying `iter` object.
func (c *cassandraIterator) scan(dest ...any) bool {
	return c.iter.Scan(dest...)
}

// numRows returns a number of rows.
// This method wraps the `NumRows` method of the underlying `iter` object.
func (c *cassandraIterator) numRows() int {
	return c.iter.NumRows()
}

// cassandraQuery implements query interface.
type cassandraQuery struct {
	query *gocql.Query
}

// exec performs a Cassandra's Query Exec.
// This method wraps the `Exec` method of the underlying `query` object.
func (c *cassandraQuery) exec() error {
	return c.query.Exec()
}

// iter returns a Cassandra iterator.
// This method wraps the `Iter` method of the underlying `query` object.
func (c *cassandraQuery) iter() iterator {
	iter := cassandraIterator{iter: c.query.Iter()}

	return &iter
}

// mapScanCAS checks a Cassandra query with an IF clause and scans the existing data into map[string]any (if any).
// This method wraps the `MapScanCAS` method of the underlying `query` object.
func (c *cassandraQuery) mapScanCAS(dest map[string]any) (applied bool, err error) {
	return c.query.MapScanCAS(dest)
}

// scanCAS checks a Cassandra query with an IF clause and scans the existing data (if any).
// This method wraps the `ScanCAS` method of the underlying `query` object.
func (c *cassandraQuery) scanCAS(dest ...any) (applied bool, err error) {
	return c.query.ScanCAS(dest)
}

// cassandraClusterConfig implements clusterConfig interface.
type cassandraClusterConfig struct {
	clusterConfig *gocql.ClusterConfig
}

func newClusterConfig(config *Config) clusterConfig {
	var c cassandraClusterConfig

	config.Hosts = strings.TrimSuffix(strings.TrimSpace(config.Hosts), ",")
	hosts := strings.Split(config.Hosts, ",")
	c.clusterConfig = gocql.NewCluster(hosts...)
	c.clusterConfig.Keyspace = config.Keyspace
	c.clusterConfig.Port = config.Port
	c.clusterConfig.Authenticator = gocql.PasswordAuthenticator{Username: config.Username, Password: config.Password}

	return &c
}

// createSession creates a Cassandra session based on the provided configuration.
// This method wraps the `CreateSession` method of the underlying `clusterConfig` object.
// It creates a new Cassandra session using the configuration options specified in `c.clusterConfig`.
//
// Returns:
//   - A `session` object representing the established Cassandra connection, or `nil` if an error occurred.
//   - An `error` object if there was a problem creating the session, or `nil` if successful.
func (c *cassandraClusterConfig) createSession() (session, error) {
	sess, err := c.clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	return &cassandraSession{session: sess}, nil
}

// cassandraSession implements session interface.
type cassandraSession struct {
	session *gocql.Session
}

// query creates a Cassandra query.
// This method wraps the `Query` method of the underlying `session` object.
func (c *cassandraSession) query(stmt string, values ...any) query {
	return &cassandraQuery{query: c.session.Query(stmt, values...)}
}

func (c *cassandraSession) newBatch(batchType gocql.BatchType) batch {
	return &cassandraBatch{batch: c.session.NewBatch(batchType)}
}

// executeBatch executes a batch operation and returns nil if successful otherwise an error is returned describing the failure.
// This method wraps the `ExecuteBatch` method of the underlying `session` object.
func (c *cassandraSession) executeBatch(b batch) error {
	gocqlBatch := b.getBatch()

	return c.session.ExecuteBatch(gocqlBatch)
}

// executeBatchCAS executes a batch operation and returns true if successful.
// This method wraps the `executeBatchCAS` method of the underlying `session` object.
func (c *cassandraSession) executeBatchCAS(b batch, dest ...any) (bool, error) {
	gocqlBatch := b.getBatch()

	applied, _, err := c.session.ExecuteBatchCAS(gocqlBatch, dest...)

	return applied, err
}

// cassandraBatch implements batch interface.
type cassandraBatch struct {
	batch *gocql.Batch
}

// Query adds the query to the batch operation.
// This method wraps the `Query` method of underlying `batch` object.
func (c *cassandraBatch) Query(stmt string, args ...any) {
	c.batch.Query(stmt, args...)
}

// getBatch returns the underlying `gocql.Batch`.
func (c *cassandraBatch) getBatch() *gocql.Batch {
	return c.batch
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
