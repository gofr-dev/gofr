package main

import (
	"log"
	"sync"
	"time"

	"gofr.dev/pkg/gofr"
)

//nolint:gochecknoglobals // used in main_test.go
var (
	n  = 0
	mu sync.RWMutex
)

const minute = 3

func main() {
	app := gofr.New()

	c := gofr.NewCron()

	// runs every minute
	err := c.AddJob("* * * * *", count)
	if err != nil {
		app.Logger.Error(err)
		return
	}

	// setting maximum duration of this program
	time.Sleep(minute * time.Minute)
}

func count() {
	mu.Lock()
	defer mu.Unlock()

	n++
	log.Println("Count: ", n)
}
