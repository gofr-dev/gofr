package gofr

import (
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/dgraph"
	"gofr.dev/pkg/gofr/datasource/file"
)

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(db container.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}

// AddFTP sets the FTP datasource in the app's container.
// Deprecated: Use the AddFile method instead.
func (a *App) AddFTP(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddFile sets the FTP,SFTP,S3 datasource in the app's container.
func (a *App) AddFileStore(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
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

// AddDgraph sets the Dgraph datasource in the app's container.
func (a *App) AddDgraph(host, port string) {
	// Create the Dgraph client with the provided configuration
	config := dgraph.Config{
		Host: host,
		Port: port,
	}

	dgraphClient := dgraph.New(config)

	// Set the logger and metrics for the Dgraph client
	dgraphClient.UseLogger(a.Logger())
	dgraphClient.UseMetrics(a.Metrics())

	// Connect to the Dgraph database
	if err := dgraphClient.Connect(); err != nil {
		a.Logger().Error("Failed to connect to Dgraph: ", err)
		return
	}

	// Add the Dgraph client to the app container
	a.container.DGraph = dgraphClient
}
