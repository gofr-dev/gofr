package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/examples/using-graphql/migrations"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func main() {
	app := gofr.New()

	// Register Migrations
	app.Migrate(migrations.All())

	// Query returning a simple string
	app.GraphQLQuery("hello", func(_ *gofr.Context) (any, error) {
		return "Hello GoFr GraphQL with SQL!", nil
	})

	// Query fetching a single user from SQL
	app.GraphQLQuery("user", func(c *gofr.Context) (any, error) {
		var user User
		err := c.SQL.QueryRowContext(c, "SELECT id, name, role FROM users LIMIT 1").Scan(&user.ID, &user.Name, &user.Role)
		if err != nil {
			return nil, err
		}

		return user, nil
	})

	// Query returning all users from SQL
	app.GraphQLQuery("users", func(c *gofr.Context) (any, error) {
		rows, err := c.SQL.QueryContext(c, "SELECT id, name, role FROM users")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var users []User
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name, &u.Role); err != nil {
				return nil, err
			}
			users = append(users, u)
		}

		return users, nil
	})

	// Query with arguments using SQL
	app.GraphQLQuery("getUser", func(c *gofr.Context) (any, error) {
		var args struct {
			ID int `json:"id"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		var user User
		err = c.SQL.QueryRowContext(c, "SELECT id, name, role FROM users WHERE id = ?", args.ID).
			Scan(&user.ID, &user.Name, &user.Role)
		if err != nil {
			return nil, err
		}

		return user, nil
	})

	// Mutation inserting into SQL
	app.GraphQLMutation("createUser", func(c *gofr.Context) (any, error) {
		var args struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		result, err := c.SQL.ExecContext(c, "INSERT INTO users (name, role) VALUES (?, ?)", args.Name, args.Role)
		if err != nil {
			return nil, err
		}

		id, _ := result.LastInsertId()

		return User{
			ID:   int(id),
			Name: args.Name,
			Role: args.Role,
		}, nil
	})

	app.Run()
}
