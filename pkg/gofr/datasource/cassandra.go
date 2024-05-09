package datasource

type Cassandra interface {
	Query(result interface{}, stmt string, values ...interface{}) error
	QueryRow(result interface{}, stmt string, values ...interface{}) error
	Exec(stmt string, values ...interface{}) error
}
