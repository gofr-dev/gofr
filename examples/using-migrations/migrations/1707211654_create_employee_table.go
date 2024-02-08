package migrations

import (
	"gofr.dev/pkg/gofr/migrations"
)

func createTableEmployee() migrations.Migration {
	return migrations.Migration{
		UP: func(m migrations.Datasource) error {
			_, err := m.DB.Exec(`create table if not exists test(id int  not null);`)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
