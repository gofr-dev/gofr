package gofr

import "gofr.dev/pkg/gofr/datasource"

// builder has been created such that dependencies for external database can be injected.
// without making the user aware for the same.
type builder interface {
	Build(o ...interface{})
}

func (a *App) AddMongo(db datasource.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}
