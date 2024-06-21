package migration

type Datasource struct {
	// TODO this should not be embedded rather it should be
	Logger

	SQL    SQL
	Redis  Redis
	PubSub PubSub
}
