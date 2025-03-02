package main

import (
	"net/http"
	
	"github.com/gofr-dev/gofr"
	"github.com/gofr-dev/gofr/pkg/errors"
)

func main() {
	app := gofr.New()
	
	// Example handler that returns validation error
	app.GET("/users/:id", func(c *gofr.Context) (interface{}, error) {
		id := c.PathParam("id")
		if id == "invalid" {
			return nil, errors.NewValidationProblem("Invalid user ID format")
		}
		
		// Simulate user not found
		if id == "999" {
			return nil, errors.NewNotFoundProblem("User not found").
				WithInstance("/users/999").
				WithExtension("userId", "999")
		}
		
		return map[string]string{"id": id, "name": "John Doe"}, nil
	})
	
	app.Start()
} 
