package scylladb

import (
	"regexp"
	"strings"

	"github.com/gocql/gocql"
)

// scylladbIterator implements iterator interface.
type scylladbIterator struct {
	iter *gocql.Iter
}

// Columns gets the Column information.
// This method wraps the `Columns` method of the underlying `iter` object.
func (s *scylladbIterator) Columns() []gocql.ColumnInfo {
	return s.iter.Columns()
}

//	Scan gets the next row from the Cassandra iterator and fills in the provided arguments.
//
// This method wraps the `Scan` method of the underlying `iter` object.
func (s *scylladbIterator) Scan(dest ...any) bool {
	return s.iter.Scan(dest...)
}

// NumRows returns a number of rows.
func (s *scylladbIterator) NumRows() int {
	return s.iter.NumRows()
}

// scylladbQuery implements query interface.
type scylladbQuery struct {
	query *gocql.Query
}

// Exec performs a ScyllaDB Query Exec.
func (s *scylladbQuery) Exec() error {
	return s.query.Exec()
}

// Iter returns a ScyllaDB iterator.
func (s *scylladbQuery) Iter() iterator {
	iter := scylladbIterator{iter: s.query.Iter()}

	return &iter
}

// MapScanCAS checks a ScyllaDB query an IF clause and scans the existing data into map[string]any (if any).
// This method wraps the `MapScanCAS` method of the underlying `query` object.
func (s *scylladbQuery) MapScanCAS(dest map[string]any) (applied bool, err error) {
	return s.query.MapScanCAS(dest)
}

// ScanCAS checks a ScyllaDB query with a IF clause and scans the existing data.
// This method wraps the `ScanCAS` method of the underlying `query` object.
func (s *scylladbQuery) ScanCAS(dest ...any) (applied bool, err error) {
	return s.query.ScanCAS(dest)
}

// scyllaClusterConfig implements clusterConfig.
type scyllaClusterConfig struct {
	clusterConfig *gocql.ClusterConfig
}

func newClusterConfig(config *Config) clusterConfig {
	var s scyllaClusterConfig

	config.Hosts = strings.TrimSuffix(strings.TrimSpace(config.Hosts), ",")
	hosts := strings.Split(config.Hosts, ",")
	s.clusterConfig = gocql.NewCluster(hosts...)
	s.clusterConfig.Keyspace = config.Keyspace
	s.clusterConfig.Port = config.Port
	s.clusterConfig.Authenticator = gocql.PasswordAuthenticator{Username: config.Username, Password: config.Password}

	return &s
}

// createSession creates a ScyllaDB session based on the provided configuration.
func (s *scyllaClusterConfig) createSession() (session, error) {
	sess, err := s.clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	return &scyllaSession{session: sess}, nil
}

// scyllaSession implements session.
type scyllaSession struct {
	session *gocql.Session
}

// Query creates a ScyllaDB query.
func (s *scyllaSession) Query(stmt string, values ...any) query {
	return &scylladbQuery{query: s.session.Query(stmt, values...)}
}

func (s *scyllaSession) newBatch(batchType gocql.BatchType) batch {
	return &scyllaBatch{batch: s.session.NewBatch(batchType)}
}

// executeBatch executes a batch operation.
func (s *scyllaSession) executeBatch(b batch) error {
	gocqlBatch := b.getBatch()

	return s.session.ExecuteBatch(gocqlBatch)
}

// executeBatchCAS executes a batch operation and returns true if successful.
func (s *scyllaSession) executeBatchCAS(batch batch, dest ...any) (bool, error) {
	gocqlBatch := batch.getBatch()

	applied, _, err := s.session.ExecuteBatchCAS(gocqlBatch, dest...)

	return applied, err
}

// scyllaBatch  implements batch.
type scyllaBatch struct {
	batch *gocql.Batch
}

// Query adds the query to the batch operation.
func (s *scyllaBatch) Query(stmt string, args ...any) {
	s.batch.Query(stmt, args...)
}

// getBatch returns the underlying `gocql.Batch`.
func (s *scyllaBatch) getBatch() *gocql.Batch {
	return s.batch
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
