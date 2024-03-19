package main

import "gofr.dev/pkg/gofr"

type user struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`

	gofr.CRUD `json:"-"`
}

// GetAll : User can overwrite the specific handlers by implementing them like this
func (u *user) GetAll(c *gofr.Context) (interface{}, error) {
	return "user GetAll called", nil
}

func main() {
	// Create a new application
	a := gofr.New()

	// CRUDFromStruct creates CRUD handles for the given entity
	err := a.CRUDFromStruct(&user{})
	if err != nil {
		return
	}

	// Run the application
	a.Run()
}
