package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func CreateUsersTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec("CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255), role VARCHAR(255))")
			return err
		},
	}
}
