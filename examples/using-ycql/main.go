package main

import (
	"gofr.dev/cmd/gofr/migration"
	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
	handler "gofr.dev/examples/using-ycql/handlers/shop"
	"gofr.dev/examples/using-ycql/migrations"
	store "gofr.dev/examples/using-ycql/stores/shop"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create the application object
	app := gofr.New()

	app.Server.ValidateHeaders = false

	migrate := map[string]dbmigration.Migrator{"20230116104833": migrations.All()["20230116104833"]}
	db := dbmigration.NewYCQL(&app.YCQL)

	err := migration.Migrate("gofr-YCQL-example", db, migrate, "UP", app.Logger)
	if err != nil {
		app.Logger.Errorf("Error:%v", err)
	}

	// initialize store dependency
	s := store.New()
	// initialize the handler
	h := handler.New(s)
	// added get handler
	app.GET("/shop", h.Get)
	// added create handler
	app.POST("/shop", h.Create)
	// added update handler
	app.PUT("/shop/{id}", h.Update)
	// added delete handler
	app.DELETE("/shop/{id}", h.Delete)
	// server  can  start at custom port
	app.Server.HTTP.Port = 8085

	// server start
	app.Start()
}
