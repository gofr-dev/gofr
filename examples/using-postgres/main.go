package main

import (
	"gofr.dev/cmd/gofr/migration"
	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
	"gofr.dev/examples/using-postgres/handler"
	"gofr.dev/examples/using-postgres/migrations"
	"gofr.dev/examples/using-postgres/store"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	s := store.New()
	h := handler.New(s)

	appName := app.Config.Get("APP_NAME")

	err := migration.Migrate(appName, dbmigration.NewGorm(app.GORM()), migrations.All(),
		dbmigration.UP, app.Logger)
	if err != nil {
		app.Logger.Error(err)

		return
	}

	// specifying the different routes supported by this service
	app.GET("/customer", h.Get)
	app.GET("/customer/{id}", h.GetByID)
	app.POST("/customer", h.Create)
	app.PUT("/customer/{id}", h.Update)
	app.DELETE("/customer/{id}", h.Delete)

	// starting the server on a custom port
	app.Server.HTTP.Port = 9092
	app.Server.MetricsPort = 2325
	app.Start()
}
