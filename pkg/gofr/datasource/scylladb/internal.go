package scylladb

import (
	"github.com/gocql/gocql"
	"regexp"
	"strings"
)

type scylladbIterator struct {
	iter *gocql.Iter
}

func (s *scylladbIterator) columns() []gocql.ColumnInfo {
	return s.iter.Columns()
}
func (s *scylladbIterator) scan(dest ...any) bool {
	return s.iter.Scan(dest...)
}

func (s *scylladbIterator) numRows() int {
	return s.iter.NumRows()
}

type scylladbQuery struct {
	query *gocql.Query
}

func (s *scylladbQuery) exec() error {
	return s.query.Exec()
}

func (s *scylladbQuery) iter() iterator {
	iter := scylladbIterator{iter: s.query.Iter()}

	return &iter
}

func (c *scylladbQuery) mapScanCAS(dest map[string]any) (applied bool, err error) {
	return c.query.MapScanCAS(dest)
}

func (c *scylladbQuery) scanCAS(dest ...any) (applied bool, err error) {
	return c.query.ScanCAS(dest)
}

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

type scyllaSession struct {
	session *gocql.Session
}

func (c *scyllaClusterConfig) createSession() (session, error) {
	sess, err := c.clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	return &scyllaSession{session: sess}, nil
}

func (c *scyllaSession) query(stmt string, values ...any) query {
	return &scylladbQuery{query: c.session.Query(stmt, values...)}
}

func (c *scyllaSession) newBatch(batchType gocql.BatchType) batch {
	return &scyllaBatch{batch: c.session.NewBatch(batchType)}
}

func (c *scyllaSession) executeBatch(b batch) error {
	gocqlBatch := b.getBatch()

	return c.session.ExecuteBatch(gocqlBatch)
}

func (c *scyllaSession) executeBatchCAS(b batch, dest ...any) (bool, error) {
	gocqlBatch := b.getBatch()

	applied, _, err := c.session.ExecuteBatchCAS(gocqlBatch, dest...)

	return applied, err
}

type scyllaBatch struct {
	batch *gocql.Batch
}

func (c *scyllaBatch) Query(stmt string, args ...any) {
	c.batch.Query(stmt, args...)
}

func (c *scyllaBatch) getBatch() *gocql.Batch {
	return c.batch
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
