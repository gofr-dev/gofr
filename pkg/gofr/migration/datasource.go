package migration

import "gofr.dev/pkg/gofr/container"

type Datasource struct {
	// TODO Logger should not be embedded rather it should be a field.
	// Need to think it through as it will bring breaking changes.
	Logger

	SQL    SQL
	Redis  Redis
	PubSub PubSub
}

// It is a base implementation for migration manger, on this other database drivers have been wrapped.

func (d Datasource) checkAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (d Datasource) getLastMigration(*container.Container) int64 {
	return 0
}

func (d Datasource) beginTransaction(*container.Container) transactionData {
	return transactionData{}
}

func (d Datasource) commitMigration(c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d Datasource) rollback(*container.Container, transactionData) {}
