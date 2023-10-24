package main

import (
	"crypto/tls"
	"net/http"

	avroHandler "gofr.dev/examples/universal-example/avro/handlers"
	cassandraHandler "gofr.dev/examples/universal-example/cassandra/handlers"
	cassandraStore "gofr.dev/examples/universal-example/cassandra/store/employee"
	eventHandler "gofr.dev/examples/universal-example/eventhub/handlers"
	svcHandler "gofr.dev/examples/universal-example/gofr-services/handler"
	svc "gofr.dev/examples/universal-example/gofr-services/service"
	pgsqlHandler "gofr.dev/examples/universal-example/pgsql/handler"
	pgsqlStore "gofr.dev/examples/universal-example/pgsql/store"
	redisHandler "gofr.dev/examples/universal-example/redis/handler"
	redisStore "gofr.dev/examples/universal-example/redis/store"
	"gofr.dev/pkg/datastore/pubsub/eventhub"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/service"
)

func main() {
	// Create the application object
	app := gofr.New()

	// To disable the header validation
	app.Server.ValidateHeaders = false

	// Service urls
	urlHelloAPI := app.Config.Get("GOFR_HELLO_API")
	urlLoggingService := app.Config.Get("GOFR_LOGGING_SERVICE")

	// Skip TLS verification
	var tr = &http.Transport{
		//nolint:gosec // need this to skip TLS verification
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Gofr-logging service
	logService := service.NewHTTPServiceWithOptions(urlLoggingService, app.Logger, nil)

	logService.Client = &http.Client{Transport: tr}
	logService.Transport = tr

	logSrv := svc.New(logService)
	loggingHandler := svcHandler.New(logSrv)
	app.GET("/level", loggingHandler.Log)

	// Gofr-hello-api service
	helloService := service.NewHTTPServiceWithOptions(urlHelloAPI, app.Logger, nil)

	helloService.Client = &http.Client{Transport: tr}
	helloService.Transport = tr

	helloSrv := svc.New(helloService)
	helloSrvHandler := svcHandler.New(helloSrv)
	app.GET("/hello", helloSrvHandler.Hello)

	// Redis
	redisStr := redisStore.New()
	redisHandle := redisHandler.New(redisStr)

	app.GET("/redis/config/{key}", redisHandle.GetKey)
	app.POST("/redis/config", redisHandle.SetKey)

	// Postgres
	pgsqlStr := pgsqlStore.New()
	pgsqlHandle := pgsqlHandler.New(pgsqlStr)

	app.GET("/pgsql/employee", pgsqlHandle.Get)
	app.POST("/pgsql/employee", pgsqlHandle.Create)

	// Cassandra
	cassandraStr := cassandraStore.New()
	cassandraHandle := cassandraHandler.New(cassandraStr)

	app.GET("/cassandra/employee", cassandraHandle.Get)
	app.POST("/cassandra/employee", cassandraHandle.Create)

	app.GET("/avro/pub", avroHandler.Producer)
	app.GET("/avro/sub", avroHandler.Consumer)

	config := eventhub.Config{
		Namespace:    app.Config.Get("EVENTHUB_NAMESPACE"),
		EventhubName: app.Config.Get("EVENTHUB_NAME"),
		ClientID:     app.Config.Get("AZURE_CLIENT_ID"),
		ClientSecret: app.Config.Get("AZURE_CLIENT_SECRET"),
		TenantID:     app.Config.Get("AZURE_TENANT_ID"),
	}

	// Eventhub
	eventHub, err := eventhub.New(&config)
	if err != nil {
		app.Logger.Errorf("Azure Eventhub could not be initialized, Namespace: %v, Eventhub: %v, error: %v\n",
			config.Namespace, config.EventhubName, err)
	}

	app.Logger.Infof("Azure Eventhub initialized, Namespace: %v, Eventhub: %v\n", config.Namespace, config.EventhubName)

	eventHandle := eventHandler.New(eventHub)
	app.GET("/eventhub/pub", eventHandle.Producer)
	app.GET("/eventhub/sub", eventHandle.Consumer)

	// HealthCheck for Services
	app.ServiceHealth = append(app.ServiceHealth, helloService.HealthCheck, logService.HealthCheck)

	// HealthCheck for EventHub
	app.DatabaseHealth = append(app.DatabaseHealth, eventHub.HealthCheck)

	// Start the server
	app.Start()
}
