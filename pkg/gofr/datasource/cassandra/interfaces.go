package cassandra

import "github.com/gocql/gocql"

type clusterConfig interface {
	CreateSession() (session, error)
}

type session interface {
	Query(stmt string, values ...interface{}) query
}

type query interface {
	Exec() error
	Iter() iterator
	MapScanCAS(dest map[string]interface{}) (applied bool, err error)
	ScanCAS(dest ...any) (applied bool, err error)
}

type iterator interface {
	Columns() []gocql.ColumnInfo
	Scan(dest ...interface{}) bool
	NumRows() int
}
