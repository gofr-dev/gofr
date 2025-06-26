package main

import (
	"gofr.dev/examples/using-add-rest-handlers/migrations"
	"gofr.dev/pkg/gofr"
)

type user struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`
}

// GetAll : User can overwrite the specific handlers by implementing them like this
func (u *user) GetAll(c *gofr.Context) (any, error) {
	return "user GetAll called", nil
}

func main() {
	// Create a new application
	a := gofr.New()

	// Add migrations to run
	a.Migrate(migrations.All())

	// AddRESTHandlers creates CRUD handles for the given entity
	err := a.AddRESTHandlers(&user{})
	if err != nil {
		return
	}

	// Run the application
	a.Run()
}
