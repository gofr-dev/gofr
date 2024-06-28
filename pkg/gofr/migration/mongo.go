package migration

import (
	"context"
	"gofr.dev/pkg/gofr/container"
)

type mongoDS struct {
	Mongo
}

type mongoMigrator struct {
	Mongo

	migrator
}

func (ch mongoDS) apply(m migrator) migrator {
	return mongoMigrator{
		Mongo:    ch.Mongo,
		migrator: m,
	}
}

//const (
//	CheckAndCreateChMigrationTable = `CREATE TABLE IF NOT EXISTS gofr_migrations
//(
//    version    Int64     NOT NULL,
//    method     String    NOT NULL,
//    start_time DateTime  NOT NULL,
//    duration   Int64     NULL,
//    PRIMARY KEY (version, method)
//) ENGINE = MergeTree()
//ORDER BY (version, method);
//`
//
//	getLastChGoFrMigration = `SELECT COALESCE(MAX(version), 0) as last_migration FROM gofr_migrations;`
//
//	insertChGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`
//)

func (m mongoMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := c.Mongo.CreateCollection(context.Background(), "gofr_migration"); err != nil {
		// TODO: Handle error properly
		return err
	}

	return m.migrator.checkAndCreateMigrationTable(c)
}

func (ch mongoMigrator) getLastMigration(c *container.Container) int64 {
	type LastMigration struct {
		Timestamp int64 `ch:"last_migration"`
	}

	var lastMigrations []LastMigration

	var lastMigration int64

	// TODO replace with Mongo
	//err := c.Clickhouse.Select(context.Background(), &lastMigrations, getLastChGoFrMigration)
	//if err != nil {
	//	return 0
	//}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	if len(lastMigrations) != 0 {
		lastMigration = lastMigrations[0].Timestamp
	}

	lm2 := ch.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (m mongoMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := m.migrator.beginTransaction(c)

	sess, err := c.Mongo.StartSession()
	if err != nil {

	}

	err = sess.StartTransaction()
	if err != nil {

	}

	if err == nil {
		c.Debug("Mongo Transaction begin successfully")
	}

	return cmt
}

func (m mongoMigrator) commitMigration(c *container.Container, data transactionData) error {
	_, err := m.Mongo.InsertOne(context.Background(), "gofr_migration")
	//data.MigrationNumber,
	//	"UP", data.StartTime, time.Since(data.StartTime).Milliseconds()
	if err != nil {
		return err
	}

	c.Debugf("inserted record for migration %v in clickhouse gofr_migrations table", data.MigrationNumber)

	err = data.MongoTx.CommitTransaction(context.Background())
	if err != nil {
		return err
	}

	data.MongoTx.EndSession(context.Background())

	return m.migrator.commitMigration(c, data)
}

func (m mongoMigrator) rollback(c *container.Container, data transactionData) {
	c.Errorf("Migration %v failed", data.MigrationNumber)

	err := data.MongoTx.AbortTransaction(context.Background())
	if err != nil {
		//TODO
	}

	data.MongoTx.EndSession(context.Background())

	m.migrator.rollback(c, data)
}
