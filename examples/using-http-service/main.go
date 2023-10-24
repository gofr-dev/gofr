package main

import (
	"net/http"
	"time"

	handlers "gofr.dev/examples/using-http-service/handlers/user"
	services "gofr.dev/examples/using-http-service/services/user"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/log"
	svc "gofr.dev/pkg/service"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	const (
		numOfRetries = 3
		httpTimeout  = 10
	)

	sampleSvc := svc.NewHTTPServiceWithOptions(
		app.Config.Get("SAMPLE_API_URL"),
		app.Logger,
		&svc.Options{NumOfRetries: numOfRetries},
	)
	sampleSvc.Timeout = httpTimeout * time.Second

	app.ServiceHealth = append(app.ServiceHealth, sampleSvc.HealthCheck)

	sampleSvc.CustomRetry = httpSvcRetry

	service := services.New(sampleSvc)
	handler := handlers.New(service)

	app.GET("/user/{name}", handler.Get)
	app.Start()
}

// httpSvcRetry is used for a custom logic of retries for the http calls made to sample api
func httpSvcRetry(logger log.Logger, err error, statusCode, attemptCount int) bool {
	if statusCode == http.StatusOK {
		return false
	}

	// any error from the http client will be retried once
	if err != nil && attemptCount < 2 {
		logger.Errorf("Retrying because of err: ", err)
		return true
	}

	//nolint:gomnd // introducing constants for attemptCount values will reduce readability.
	switch attemptCount {
	case 1:
		time.Sleep(2 * time.Second)
	case 2:
		time.Sleep(4 * time.Second)
	case 3:
		time.Sleep(8 * time.Second)
	}

	return true
}
