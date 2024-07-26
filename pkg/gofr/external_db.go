package gofr

import (
	"gofr.dev/pkg/gofr/container"
)

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(db container.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package : gofr.dev/pkg/gofr/datasource/clickhouse .
func (a *App) AddClickhouse(db container.ClickhouseProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Clickhouse = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the AddMongo method instead.
func (a *App) UseMongo(db container.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(db container.CassandraProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Cassandra = db
}

// AddKVStore sets the KV-Store datasource in the app's container.
func (a *App) AddKVStore(db container.KVStoreProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.KVStore = db
}
