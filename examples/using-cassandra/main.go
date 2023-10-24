package main

import (
	"gofr.dev/cmd/gofr/migration"
	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
	handlers "gofr.dev/examples/using-cassandra/handlers/person"
	"gofr.dev/examples/using-cassandra/migrations"
	stores "gofr.dev/examples/using-cassandra/stores/person"
	"gofr.dev/pkg/gofr"
)

func main() {
	// create the application object
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	migrate := map[string]dbmigration.Migrator{"20230113160630": migrations.All()["20230113160630"]}
	db := dbmigration.NewCassandra(&app.Cassandra)

	err := migration.Migrate("gofr-cassandra-example", db, migrate, "UP", app.Logger)
	if err != nil {
		app.Logger.Errorf("Error:%v", err)
	}

	s := stores.New()
	h := handlers.New(s)
	// add get handler
	app.GET("/persons", h.Get)
	// add post handler
	app.POST("/persons", h.Create)
	// add a delete handler
	app.DELETE("/persons/{id}", h.Delete)
	// add a put handler
	app.PUT("/persons/{id}", h.Update)

	// starting the server on a custom port
	app.Server.HTTP.Port = 9094
	app.Server.MetricsPort = 2123
	// start the server
	app.Start()
}
