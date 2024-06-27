package main

import (
	"sync"
	"time"

	"gofr.dev/pkg/gofr"
)

var (
	n  = 0
	mu sync.RWMutex
)

const duration = 3

func main() {
	app := gofr.New()

	// runs every minute
	app.AddCronJob("* * * * *", "counter", count)

	// setting the maximum duration of this application
	time.Sleep(duration * time.Minute)

	// not running the app to close after we have completed the crons running
	// since this is an example the cron will not be running forever
	// to run cron forever, users can start the metric server or normal HTTP server
	// app.Run()
}

func count(c *gofr.Context) {
	mu.Lock()
	defer mu.Unlock()

	n++
	c.Log("Count: ", n)
}
