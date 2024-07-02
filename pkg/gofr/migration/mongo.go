package migration

import (
	"context"
	"fmt"
	"gofr.dev/pkg/gofr/container"
	"strings"
	"time"
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

type mongoData struct {
	Method          string    `json:"method"`
	Duration        int64     `json:"duration"`
	StartTime       time.Time `json:"startTime"`
	MigrationNumber int64     `json:"migrationNumber"`
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
	if err := c.Mongo.CreateCollection(context.Background(), "gofr_migration"); err != nil && !strings.Contains(fmt.Sprint(err), "gofr_migration already exists") {
		fmt.Println(err)
		return err
	}

	return m.migrator.checkAndCreateMigrationTable(c)
}

func (ch mongoMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigrations []mongoData

	err := c.Mongo.Find(context.Background(), "gofr_migration", nil, &lastMigrations)
	if err != nil {
		return 0
	}

	c.Debugf("Mongo last migration fetched value is: %v", lastMigrations)

	var lm int64

	for _, v := range lastMigrations {
		if v.MigrationNumber > lm {
			lm = v.MigrationNumber
		}
	}

	fetchedLastMigrations := ch.migrator.getLastMigration(c)

	if fetchedLastMigrations > lm {
		return fetchedLastMigrations
	}

	return lm
}

func (m mongoMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := m.migrator.beginTransaction(c)

	sess, err := c.Mongo.StartSession()
	if err != nil {
		c.Error("unable to start session for mongoDB: %v", err)

		return cmt
	}

	ses, ok := sess.(container.Transaction)
	if !ok {
		c.Error("unable to start session for mongoDB transaction due to driver error: %v", err)

		return cmt
	}

	err = ses.StartTransaction()
	if err != nil {
		c.Error("unable to start transaction for mongoDB: %v", err)

		return cmt
	}

	cmt.MongoTx = ses

	c.Debug("Mongo Transaction begin successfully")

	return cmt
}

func (m mongoMigrator) commitMigration(c *container.Container, data transactionData) error {
	type mongoData struct {
		Method          string    `json:"method"`
		Duration        int64     `json:"duration"`
		StartTime       time.Time `json:"startTime"`
		MigrationNumber int64     `json:"migrationNumber"`
	}

	_, err := m.Mongo.InsertOne(context.Background(), "gofr_migration", mongoData{
		MigrationNumber: data.MigrationNumber, Duration: time.Since(data.StartTime).Milliseconds(),
		StartTime: data.StartTime, Method: "UP"})
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
		c.Error("unable to rollback transaction: %v", err)
	}

	data.MongoTx.EndSession(context.Background())

	c.Errorf("Migration %v failed and rolled back", data.MigrationNumber)

	m.migrator.rollback(c, data)
}
