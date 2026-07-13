package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.GET("/products/{id}", getProduct)
	app.POST("/products", createProduct)

	// Every read-only handler becomes an MCP tool an agent can call, on port 8200 (MCP_PORT).
	// Pass gofr.WithWriteTools() to also expose createProduct.
	app.EnableMCP()

	app.Run()
}

func getProduct(c *gofr.Context) (any, error) {
	return map[string]string{"id": c.PathParam("id"), "name": "widget"}, nil
}

func createProduct(c *gofr.Context) (any, error) {
	var in struct {
		Name string `json:"name"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	return map[string]string{"status": "created", "name": in.Name}, nil
}
