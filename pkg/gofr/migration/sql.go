package migration

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

const (
	createSQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	createSQLGoFrMigrationsTableMSSQL = `IF NOT EXISTS (SELECT * FROM sys.objects 
	WHERE object_id = OBJECT_ID(N'gofr_migrations') AND type = N'U')
    BEGIN
        CREATE TABLE gofr_migrations (
                                         version BIGINT NOT NULL,
                                         method VARCHAR(4) NOT NULL,
                                         start_time DATETIME2 NOT NULL,
                                         duration BIGINT,
                                         CONSTRAINT PK_gofr_migrations PRIMARY KEY (version, method)
        );
    END;
`

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`

	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`

	insertGoFrMigrationRowMSSQL = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`

	dialectMySQL    = "mysql"
	dialectMSSQL    = "dialectMSSQL"
	dialectSQLite   = "dialectSQLite"
	dialectPostgres = "dialectPostgres"
)

// database/sql is the package imported so named it sqlDS.
type sqlDS struct {
	SQL
}

func (s *sqlDS) apply(m migrator) migrator {
	return sqlMigrator{
		SQL:      s.SQL,
		migrator: m,
	}
}

type sqlMigrator struct {
	SQL

	migrator
}

func (d sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	var createTableQuery string

	switch c.SQL.Dialect() {
	case dialectMySQL, dialectSQLite, dialectPostgres:
		createTableQuery = createSQLGoFrMigrationsTable
	case dialectMSSQL:
		createTableQuery = createSQLGoFrMigrationsTableMSSQL
	}

	if _, err := c.SQL.Exec(createTableQuery); err != nil {
		return err
	}

	return d.migrator.checkAndCreateMigrationTable(c)
}

func (d sqlMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	lm2 := d.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d sqlMigrator) commitMigration(c *container.Container, data transactionData) error {
	switch c.SQL.Dialect() {
	case dialectMySQL, dialectSQLite:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMySQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)

	case dialectPostgres:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowPostgres, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)
	case dialectMSSQL:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMSSQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)
	}

	// Commit transaction
	if err := data.SQLTx.Commit(); err != nil {
		return err
	}

	return d.migrator.commitMigration(c, data)
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func (d sqlMigrator) beginTransaction(c *container.Container) transactionData {
	sqlTx, err := c.SQL.Begin()
	if err != nil {
		c.Errorf("unable to begin transaction: %v", err)

		return transactionData{}
	}

	cmt := d.migrator.beginTransaction(c)

	cmt.SQLTx = sqlTx

	c.Debug("SQL Transaction begin successful")

	return cmt
}

func (d sqlMigrator) rollback(c *container.Container, data transactionData) {
	if data.SQLTx == nil {
		return
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	d.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}
