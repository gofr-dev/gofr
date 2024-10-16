package gofr

import (
	"context"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
)

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(ctx context.Context, db container.MongoProvider) error {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-mongo")

	db.UseTracer(tracer)

	if err := db.Connect(ctx); err != nil {
		return err
	}

	a.container.Mongo = db

	return nil
}

// AddFTP sets the FTP datasource in the app's container.
// Deprecated: Use the AddFile method instead.
func (a *App) AddFTP(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddPubSub sets the PubSub client in the app's container.
func (a *App) AddPubSub(ctx context.Context, pubsub container.PubSubProvider) error {
	pubsub.UseLogger(a.Logger())
	pubsub.UseMetrics(a.Metrics())

	if err := pubsub.Connect(ctx); err != nil {
		return err
	}

	a.container.PubSub = pubsub

	return nil
}

// AddFileStore sets the FTP,SFTP,S3 datasource in the app's container.
func (a *App) AddFileStore(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package : gofr.dev/pkg/gofr/datasource/clickhouse .
func (a *App) AddClickhouse(ctx context.Context, db container.ClickhouseProvider) error {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-clickhouse")

	db.UseTracer(tracer)

	if err := db.Connect(ctx); err != nil {
		a.Logger().Error("Failed to connect to Clickhouse", err)
		return err
	}

	a.container.Clickhouse = db

	return nil
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the AddMongo method instead.
func (a *App) UseMongo(db container.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(ctx context.Context, db container.CassandraProvider) error {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-cassandra")

	db.UseTracer(tracer)

	if err := db.Connect(ctx); err != nil {
		return err
	}

	a.container.Cassandra = db

	return nil
}

// AddKVStore sets the KV-Store datasource in the app's container.
func (a *App) AddKVStore(ctx context.Context, db container.KVStoreProvider) error {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	if err := db.Connect(ctx); err != nil {
		return err
	}

	a.container.KVStore = db

	return nil
}

// AddSolr sets the Solr datasource in the app's container.
func (a *App) AddSolr(ctx context.Context, db container.SolrProvider) error {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	if err := db.Connect(ctx); err != nil {
		return err
	}

	a.container.Solr = db

	return nil
}

// AddDgraph sets the Dgraph datasource in the app's container.
func (a *App) AddDgraph(ctx context.Context, db container.DgraphProvider) error {
	// Create the Dgraph client with the provided configuration
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	if err := db.Connect(ctx); err != nil {
		return err
	}

	a.container.DGraph = db

	return nil
}
