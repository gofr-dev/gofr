package main

import (
	addroute "gofr.dev/cmd/gofr/addRoute"
	"gofr.dev/cmd/gofr/dockerize"
	"gofr.dev/cmd/gofr/entity"
	"gofr.dev/cmd/gofr/initialize"
	"gofr.dev/cmd/gofr/migration/handler"
	"gofr.dev/cmd/gofr/test"
	"gofr.dev/pkg/gofr"
)

func main() {
	k := gofr.NewCMD()

	dockerHandler := dockerize.New(k.Config.GetOrDefault("APP_NAME", "gofr"),
		k.Config.GetOrDefault("APP_VERSION", "dev"))

	k.GET("migrate create", handler.CreateMigration)
	k.GET("migrate", handler.Migrate)
	k.GET("dockerize run", dockerHandler.Run)
	k.GET("dockerize", dockerHandler.Dockerize)
	k.GET("init", initialize.Init)
	k.GET("entity", entity.AddEntity)
	k.GET("add", addroute.AddRoute)
	k.GET("help", helpHandler)
	k.GET("test", test.GenerateIntegrationTest)

	k.Start()
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
