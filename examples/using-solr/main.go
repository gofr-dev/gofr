package main

import (
	"bytes"
	"encoding/json"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/solr"
)

func main() {
	app := gofr.New()

	app.AddSolr(solr.New(solr.Config{
		Host: "localhost",
		Port: "2020",
	}))

	app.POST("/solr", post)
	app.GET("/solr", get)

	app.Run()
}

func post(c *gofr.Context) (interface{}, error) {
	p := struct {
		Name string
		Age  int
	}{"Srijan", 24}

	body, _ := json.Marshal(p)

	resp, err := c.Solr.Create(c, "test", bytes.NewBuffer(body), nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func get(c *gofr.Context) (interface{}, error) {
	resp, err := c.Solr.Search(c, "test", nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
