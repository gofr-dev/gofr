package migration

import "gofr.dev/pkg/gofr/container"

// keeping the migrator interface unexported as, right now it is not being implemented directly, by the externalDB drivers.
// keeping the implementations at one place such that if any change in migration logic, we would change directly here.
// it uses the interface defined in datasource package.
type migrator interface {
	checkAndCreateMigrationTable(c *container.Container) error
	getLastMigration(c *container.Container) int64

	beginTransaction(c *container.Container) transactionData

	commitMigration(c *container.Container, data transactionData) error
	rollback(c *container.Container, data transactionData)
}

// It is a base implementation for migration manger, on this other database drivers have been wrapped.
// This could have been implemented on Datasource type as well, but then user would have got access to these methods,
// which was of no use to the users.

type manager struct {
}

func (d manager) checkAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (d manager) getLastMigration(*container.Container) int64 {
	return 0
}

func (d manager) beginTransaction(*container.Container) transactionData {
	return transactionData{}
}

func (d manager) commitMigration(c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d manager) rollback(*container.Container, transactionData) {}
