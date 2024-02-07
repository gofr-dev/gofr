package migrations

import (
	"gofr.dev/pkg/gofr/migrations"
)

func createTableEmployee() migrations.Migration {
	return migrations.Migration{
		UP: func(m migrations.Datasource) error {
			_, err := m.DB.Exec(`create table test(hello int null);`)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
