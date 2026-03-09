package main

import (
	"gofr.dev/examples/using-graphql/migrations"
	"gofr.dev/pkg/gofr"
)

// User is the domain type used in GraphQL resolvers and integration tests.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func main() {
	app := gofr.New()

	app.Migrate(migrations.All())

	// Example: curl -X POST http://localhost:9091/graphql -H "Content-Type: application/json" -d '{"query": "{ hello }"}'
	app.GraphQLQuery("hello", func(c *gofr.Context) (any, error) {
		return "Hello GoFr GraphQL!", nil
	})

	// Example: curl -X POST -H "Content-Type: application/json" -d '{"query": "query GetUser($id: Int) { getUser(id: $id) { name role } }", "variables": {"id": 1}}' http://localhost:9091/graphql
	app.GraphQLQuery("getUser", func(c *gofr.Context) (any, error) {
		var args struct {
			ID int `json:"id"`
		}

		if err := c.Bind(&args); err != nil {
			return nil, err
		}

		var user User

		err := c.SQL.QueryRowContext(c, "SELECT id, name, role FROM users WHERE id = ?", args.ID).
			Scan(&user.ID, &user.Name, &user.Role)
		if err != nil {
			return nil, err
		}

		return user, nil
	})

	// Example: curl -X POST -H "Content-Type: application/json" -d '{"query": "mutation CreateUser($name: String, $role: String) { createUser(name: $name, role: $role) { id name } }", "variables": {"name": "New User", "role": "admin"}}' http://localhost:9091/graphql
	app.GraphQLMutation("createUser", func(c *gofr.Context) (any, error) {
		var args struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}

		if err := c.Bind(&args); err != nil {
			return nil, err
		}

		result, err := c.SQL.ExecContext(c, "INSERT INTO users (name, role) VALUES (?, ?)", args.Name, args.Role)
		if err != nil {
			return nil, err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}

		return User{ID: int(id), Name: args.Name, Role: args.Role}, nil
	})

	app.Run()
}
