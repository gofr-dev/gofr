package datasource

type Cassandra interface {
	Query(stmt string, values ...interface{}) Query
	Iter(stmt string, values ...interface{}) Iter
}

type Query interface {
	Keyspace() string
	Table() string
	Exec() error
	Scan(dest ...interface{}) error
	ScanCAS(dest ...interface{}) (applied bool, err error)
	MapScanCAS(dest map[string]interface{}) (applied bool, err error)
}

type Iter interface {
	Scan(dest ...interface{}) bool
	NumRows() int
	Close() error
}
