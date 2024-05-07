package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

const createTable = `CREATE TABLE IF NOT EXISTS user (
    id 			int 		not null primary key,
    name 		varchar(50) not null,
    age 		int 		not null,
    is_employed 	bool 		not null
);`

func createTableUser() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(createTable)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
