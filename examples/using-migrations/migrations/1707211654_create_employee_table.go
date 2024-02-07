package migrations

import (
	"gofr.dev/pkg/gofr/migrations"
)

func createTableEmployee() migrations.Migration {
	return migrations.Migration{
		UP: func(m migrations.Migrator) error {
			_, err := m.SQL.Exec(`create table test(hello int null);`)
			if err != nil {
				return err
			}

			return nil
		},
		DOWN: func(m migrations.Migrator) error {
			return nil
		},
	}
}
