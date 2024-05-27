package gofr

import "gofr.dev/pkg/gofr/datasource"

func (a *App) AddMongo(db datasource.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the NewMongo function AddMongo instead.
func (a *App) UseMongo(db datasource.Mongo) {
	a.container.Mongo = db
}
