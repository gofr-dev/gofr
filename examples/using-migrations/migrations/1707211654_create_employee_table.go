package migrations

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

func createTableEmployee() gofr.Migration {
	return gofr.Migration{
		UP: func(container container.Container) error {
			return nil
		},
		DOWN: func(container container.Container) error {
			return nil
		},
	}
}
