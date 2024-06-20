package migration

import "gofr.dev/pkg/gofr/container"

type Manager interface {
	CheckAndCreateMigrationTable(c *container.Container) error
	GetLastMigration(c *container.Container) int64

	BeginTransaction(c *container.Container) transactionData

	CommitMigration(c *container.Container, data transactionData) error
	Rollback(c *container.Container, data transactionData)
}

// It is a base implementation for redisData manger, on this other database drivers have been wrapped.
// This could have been implemented on Datasource type as well, but then user would have got access to these methods,
// which was of no use to the users.

type manager struct {
}

func (d manager) CheckAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (d manager) GetLastMigration(*container.Container) int64 {
	return 0
}

func (d manager) BeginTransaction(*container.Container) transactionData {
	return transactionData{}
}

func (d manager) CommitMigration(c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d manager) Rollback(*container.Container, transactionData) {}
