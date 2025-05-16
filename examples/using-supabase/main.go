package main

import (
	"gofr.dev/pkg/gofr"
)

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func main() {
	// Initialize GoFr app
	app := gofr.New()

	app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {
		// Assuming a users table in the Supabase database
		rows, err := ctx.DB().QueryContext(ctx, "SELECT id, first_name, last_name, email FROM users LIMIT 10")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var users []User
		for rows.Next() {
			var user User
			if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email); err != nil {
				return nil, err
			}
			users = append(users, user)
		}

		return users, nil
	})

	app.Start()
}