package gofr

import "gofr.dev/pkg/gofr/datasource"

func (a *App) AddMongo(db datasource.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package : gofr.dev/pkg/gofr/datasource/clickhouse .
func (a *App) AddClickhouse(db datasource.ClickhouseProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Clickhouse = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the NewMongo function AddMongo instead.
func (a *App) UseMongo(db datasource.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(db datasource.CassandraProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Cassandra = db
}
