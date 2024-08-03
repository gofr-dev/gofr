package migrations

import (
	"context"

	"gofr.dev/pkg/gofr/migration"
)

func addEmployeeInRedis() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			err := d.Redis.Set(context.Background(), "Umang", "0987654321", 0).Err()
			if err != nil {
				return err
			}

			return nil

		},
	}
}
