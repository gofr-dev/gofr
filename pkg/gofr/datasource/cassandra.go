package datasource

type Cassandra interface {
	Query(dest interface{}, stmt string, values ...interface{}) error
	Exec(stmt string, values ...interface{}) error
	QueryCAS(dest interface{}, stmt string, values ...interface{}) (bool, error)
}
