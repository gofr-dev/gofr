package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

func K1_createTableEmployee() migration.Migrate {
	return migration.Migrate{
		UP: func(m migration.Datasource) error {
			_, err := m.DB.Exec(`create table if not exists test(id int  not null);`)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
