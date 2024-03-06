package migrations

import (
	"context"

	"gofr.dev/pkg/gofr/migration"
)

func createTopicsForStore() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			err := d.PubSub.CreateTopic(context.Background(), "products")
			if err != nil {
				return err
			}

			return d.PubSub.CreateTopic(context.Background(), "order-logs")
		},
	}
}
