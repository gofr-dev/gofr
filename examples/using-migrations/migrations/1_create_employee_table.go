package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

const createTable = `create table employee
(
    id             int         not null
        primary key,
    name           varchar(50) not null,
    gender         varchar(6)  not null,
    contact_number varchar(10) not null
);`

func createTableEmployee() migration.Migrate {
	return migration.Migrate{
		UP: func(m migration.Datasource) error {
			_, err := m.DB.Exec(createTable)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
