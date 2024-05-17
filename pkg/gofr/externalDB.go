package gofr

import "gofr.dev/pkg/gofr/datasource"

// builder has been created such that dependencies for external database can be injected.
// without making the user aware for the same.
type builder interface {
	Build(o ...interface{})
}

func (a *App) UseMongo(db datasource.Mongo) {
	mongo, ok := db.(builder)
	if ok {
		mongo.Build(a.Config, a.Logger(), a.Metrics())
	}

	a.container.Mongo = db
}
