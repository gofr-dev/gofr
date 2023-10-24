package main

import (
	"os"

	"gofr.dev/examples/using-solr/handler"
	"gofr.dev/examples/using-solr/store/customer"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	// initializing the solr client for core layer
	client := datastore.NewSolrClient(os.Getenv("SOLR_HOST"), os.Getenv("SOLR_PORT"))
	customerCore := customer.New(client)
	customerConsumer := handler.New(customerCore)

	// Specifying the different routes supported by this service
	app.GET("/customer", customerConsumer.List)
	app.POST("/customer", customerConsumer.Create)
	app.PUT("/customer", customerConsumer.Update)
	app.DELETE("/customer", customerConsumer.Delete)

	app.Start()
}
