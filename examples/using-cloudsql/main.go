// Package main demonstrates connecting a GoFr application to Google Cloud SQL
// using the cloudsql datasource. On GCP the connection uses IAM database
// authentication (no static password); locally you can use built-in
// username/password auth by setting DB_IAM_AUTH=false. Configuration is read from
// environment variables — see configs/.env.
//
// The cloudsql datasource itself supports both Postgres and MySQL, but this example
// is written for Postgres: it uses a "$1" placeholder and a SERIAL schema. To run it
// against MySQL, switch the placeholder to "?" and the schema to AUTO_INCREMENT.
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/cloudsql"
)

type customer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := gofr.New()

	// One call handles both environments. With DB_IAM_AUTH=true (GCP) it connects
	// to Cloud SQL using IAM auth; otherwise it uses a standard username/password
	// SQL connection (local). The code is identical either way — only configuration
	// changes — and app.SQL()/ctx.SQL keep all the usual GoFr SQL features.
	app.AddSQLDB(cloudsql.New(app.Config))

	app.GET("/customers", listCustomers)
	app.POST("/customers", addCustomer)

	app.Run()
}

func listCustomers(ctx *gofr.Context) (any, error) {
	rows, err := ctx.SQL.QueryContext(ctx, "SELECT id, name FROM customers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	customers := make([]customer, 0)

	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}

		customers = append(customers, c)
	}

	return customers, rows.Err()
}

func addCustomer(ctx *gofr.Context) (any, error) {
	var c customer
	if err := ctx.Bind(&c); err != nil {
		return nil, err
	}

	// Postgres placeholder; for MySQL use "?" instead of "$1".
	_, err := ctx.SQL.ExecContext(ctx, "INSERT INTO customers (name) VALUES ($1)", c.Name)
	if err != nil {
		return nil, err
	}

	return "customer added", nil
}
