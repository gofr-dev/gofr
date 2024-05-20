package gofr

import "gofr.dev/pkg/gofr/datasource"

func (a *App) AddMongo(db datasource.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}
