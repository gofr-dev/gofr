package cassandra

import "github.com/gocql/gocql"

type clusterConfig interface {
	CreateSession() (*gocql.Session, error)
}

type session interface {
	Query(stmt string, values ...interface{}) *gocql.Query
}

type query interface {
	Exec() error
	Iter() *gocql.Iter
	Scan(dest ...interface{}) error
	MapScanCAS(dest map[string]interface{}) (applied bool, err error)
}

type iterator interface {
	Columns() []gocql.ColumnInfo
	Scan(dest ...interface{}) bool
	NumRows() int
}
