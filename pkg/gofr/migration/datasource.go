package migration

import "gofr.dev/pkg/gofr/container"

type Datasource struct {
	// TODO Logger should not be embedded rather it should be a field.
	// Need to think it through as it will bring breaking changes.
	Logger

	SQL        SQL
	Redis      Redis
	PubSub     PubSub
	Clickhouse Clickhouse
}

// It is a base implementation for migration manger, on this other database drivers have been wrapped.

// Datasource has to be changed to a pointer in a different PR.
func (Datasource) checkAndCreateMigrationTable(*container.Container) error {
	return nil
}

// Datasource has to be changed to a pointer in a different PR.
func (Datasource) getLastMigration(*container.Container) int64 {
	return 0
}

// Datasource has to be changed to a pointer in a different PR.
func (Datasource) beginTransaction(*container.Container) transactionData {
	return transactionData{}
}

// Datasource has to be changed to a pointer in a different PR.
func (Datasource) commitMigration(c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

// Datasource has to be changed to a pointer in a different PR.
func (Datasource) rollback(*container.Container, transactionData) {}
