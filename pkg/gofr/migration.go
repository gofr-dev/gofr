package gofr

import "gofr.dev/pkg/gofr/container"

type MigrateFunc func(container container.Container) error

type Migration struct {
	UP   MigrateFunc
	DOWN MigrateFunc
}

type Driver interface {
	LastRunVersion() int64
	Run() error
}

func (a *App) Migrate(db Driver, migrations map[int64]Migration) {
	for _, value := range migrations {
		value.UP(*a.container)
	}

}
