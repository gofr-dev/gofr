package migration

type Datasource struct {
	// TODO Logger should not be embedded rather it should be a field.
	// Need to think it through as it will bring breaking changes.
	Logger

	SQL    SQL
	Redis  Redis
	PubSub PubSub
}
