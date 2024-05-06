package datasource

type Cassandra interface {
	Query(iter interface{}, stmt string, values ...interface{}) error
	Exec(stmt string, values ...interface{}) error
	Close()
}
