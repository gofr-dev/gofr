package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func createEmployeeTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS employee (
				id int not null primary key,
				name varchar(50) not null
			);`)
			return err
		},
	}
}
