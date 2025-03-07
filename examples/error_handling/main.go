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
		
		// Example 1: Using factory function with additional options
		if id == "invalid" {
			return nil, errors.NewValidationProblem(
				"Invalid user ID format",
				errors.WithInstance("/users/invalid"),
				errors.WithExtension("userId", id),
			)
		}
		
		// Example 2: Using factory function with no additional options
		if id == "999" {
			return nil, errors.NewNotFoundProblem("User not found")
		}
		
		// Example 3: Direct creation with functional options
		if id == "unauthorized" {
			return nil, errors.NewProblemDetails(
				errors.WithType("https://api.example.com/problems/custom"),
				errors.WithStatus(http.StatusForbidden),
				errors.WithTitle("Access Denied"),
				errors.WithDetail("You don't have permission to access this user"),
				errors.WithInstance("/users/unauthorized"),
				errors.WithExtension("requiredRole", "admin"),
			)
		}
		
		return map[string]string{"id": id, "name": "John Doe"}, nil
	})
	
	app.Start()
} 
