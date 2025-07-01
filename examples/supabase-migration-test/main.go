package main

import (
	"log"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/migration"
)

// Sample migration for testing
const createTestTable = `CREATE TABLE IF NOT EXISTS test_supabase_migration (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

func createTestTableMigration() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(createTestTable)
			if err != nil {
				log.Printf("Error creating test table: %v", err)
				return err
			}
			log.Println("Test table created successfully")
			return nil
		},
	}
}

func main() {
	app := gofr.New()

	// Define migrations map
	migrations := map[int64]migration.Migrate{
		20250101120000: createTestTableMigration(),
	}

	// Run migrations
	log.Println("Starting Supabase migration test...")
	app.Migrate(migrations)

	log.Println("Supabase migration test completed successfully!")
}
