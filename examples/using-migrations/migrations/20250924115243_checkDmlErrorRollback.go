package migrations

import (
	"context"
	"fmt"
	"gofr.dev/pkg/gofr/migration"
)

func checkDmlErrorRollback() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			var results []map[string]any

			// Check that the table exists (DDL auto-committed)
			if err := d.Oracle.Select(context.Background(), &results, `
             SELECT table_name FROM user_tables WHERE table_name = 'DML_ERROR_TEST2'`); err != nil {
				return fmt.Errorf("error checking table existence: %v", err)
			}
			if len(results) == 0 {
				fmt.Println("❌ DML_ERROR_TEST table does not exist (unexpected)")
			} else {
				fmt.Println("✅ DML_ERROR_TEST table exists (DDL persisted)")
			}

			// Check that no rows were inserted (INSERT should have rolled back)
			if err := d.Oracle.Select(context.Background(), &results, `
             SELECT COUNT(*) AS cnt FROM dml_error_test2`); err != nil {
				return fmt.Errorf("error checking row count: %v", err)
			}
			count := fmt.Sprintf("%v", results[0]["CNT"])
			if count == "0" {
				fmt.Println("✅ No rows found – DML was rolled back after error")
			} else {
				fmt.Printf("❌ Found %s rows – DML was NOT rolled back\n", count)
			}

			return nil
		},
	}
}
