package main

import (
	"gofr.dev/examples/using-cron-jobs/migrations"
	"gofr.dev/pkg/gofr"
	"sync"
	"time"
)

var (
	n  = 0
	mu sync.RWMutex
)

const minute = 3

type user struct {
	id   int
	name string
	age  int
}

func main() {
	app := gofr.New()

	// Add migrations to run
	app.Migrate(migrations.All())

	// runs every minute
	app.AddCronJob("* * * * *", "counter", count)

	// setting the maximum duration of this application
	time.Sleep(minute * time.Minute)

	// not running the app to close after we have completed the crons runnning
	// since this is an example the cron will not be running forever
	// to run cron forever, users can start the metric server or normal http server
	// app.Run()
}

func count(c *gofr.Context) {
	mu.Lock()
	defer mu.Unlock()

	n++
	c.Log("Count: ", n)
}
