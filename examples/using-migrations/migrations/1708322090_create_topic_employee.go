package migrations

import (
	"context"
	"gofr.dev/pkg/gofr/migration"
)

func createTopicEmployee() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {

			return d.PubSub.CreateTopic(context.Background(), "employee")
		},
	}
}
