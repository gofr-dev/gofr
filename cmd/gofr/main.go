package main

import (
	"gofr.dev/cmd/gofr/dockerize"
	"gofr.dev/cmd/gofr/entity"
	"gofr.dev/cmd/gofr/initialize"
	"gofr.dev/cmd/gofr/migration/handler"
	"gofr.dev/cmd/gofr/test"
	"gofr.dev/pkg/gofr"

	addroute "gofr.dev/cmd/gofr/addRoute"
)

func main() {
	g := gofr.NewCMD()

	dockerHandler := dockerize.New(g.Config.GetOrDefault("APP_NAME", "gofr"),
		g.Config.GetOrDefault("APP_VERSION", "dev"))

	g.GET("migrate create", handler.CreateMigration)
	g.GET("migrate", handler.Migrate)
	g.GET("dockerize run", dockerHandler.Run)
	g.GET("dockerize", dockerHandler.Dockerize)
	g.GET("init", initialize.Init)
	g.GET("entity", entity.AddEntity)
	g.GET("add", addroute.AddRoute)
	g.GET("help", helpHandler)
	g.GET("test", test.GenerateIntegrationTest)

	g.Start()
}

func helpHandler(_ *gofr.Context) (interface{}, error) {
	return `Available Commands
init
entity
add
test
migrate
migrate create
dockerize
dockerize run

Run gofr <command_name> -h for help of the command`, nil
}
