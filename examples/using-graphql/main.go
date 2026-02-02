package main

import (
	"gofr.dev/pkg/gofr"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func main() {
	app := gofr.New()

	// Query returning a simple string
	app.GraphQLQuery("hello", func(c *gofr.Context) (string, error) {
		return "Hello GoFr GraphQL with Reflection!", nil
	})

	// Query returning a struct (will be automatically reflected)
	app.GraphQLQuery("user", func(c *gofr.Context) (User, error) {
		return User{
			ID:   1,
			Name: "GoFr Developer",
			Role: "Maintainer",
		}, nil
	})

	// Query returning a list of structs
	app.GraphQLQuery("users", func(c *gofr.Context) ([]User, error) {
		return []User{
			{ID: 1, Name: "Alice", Role: "Admin"},
			{ID: 2, Name: "Bob", Role: "User"},
		}, nil
	})

	// Query with arguments using c.Bind
	app.GraphQLQuery("getUser", func(c *gofr.Context) (User, error) {
		var args struct {
			ID int `json:"id"`
		}

		_ = c.Bind(&args)

		if args.ID == 2 {
			return User{ID: 2, Name: "Bob", Role: "User"}, nil
		}

		return User{ID: 1, Name: "Alice", Role: "Admin"}, nil
	})

	app.Run()
}
