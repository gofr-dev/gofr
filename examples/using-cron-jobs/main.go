package main

import (
	"sync"

	"gofr.dev/pkg/gofr"
)

var (
	n  = 0
	mu sync.RWMutex
)

const duration = 3

func main() {
	app := gofr.New()

	// runs every second
	app.AddCronJob("* * * * * *", "counter", count)

	app.Run()
}

func count(c *gofr.Context) {
	mu.Lock()
	defer mu.Unlock()

	n++

	c.Log("Count:", n)
}
