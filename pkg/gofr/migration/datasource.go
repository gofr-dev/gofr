package migration

type Datasource struct {
	Logger

	SQL    db
	Redis  commands
	PubSub client
}
