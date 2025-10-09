package migrations

import (
	"context"
	"errors"
	"gofr.dev/pkg/gofr/migration"
)

func testDmlErrorRollback() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			// 1) Create a fresh table for testing
			if err := d.Oracle.Exec(context.Background(), `
             CREATE TABLE dml_error_test2 (
                id NUMBER(10) PRIMARY KEY,
                info VARCHAR2(100)
             )`); err != nil {
				return err
			}

			// 2) Insert a row (this is DML and should be transactional)
			if err := d.Oracle.Exec(context.Background(), `
             INSERT INTO dml_error_test2 (id, info)
             VALUES (1, 'This should be rolled back')`); err != nil {
				return err
			}

			// 3) Execute an invalid SQL to force an error
			if err := d.Oracle.Exec(context.Background(), `
             INVALID COMMAND TO TRIGGER ERROR`); err != nil {
				// Return this error so the framework rolls back the INSERT
				return errors.New("intentional SQL error to test DML rollback")
			}

			// This should never be reached
			return nil
		},
	}
}
