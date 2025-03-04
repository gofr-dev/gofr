package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

func main() {
	app := gofr.New()
	app.GET("/list", listHandler)
	app.AddStaticFiles("/", "./static")
	app.Run()
}

type Todo struct {
	Title string
	Done  bool
}

type TodoPageData struct {
	PageTitle string
	Todos     []Todo
}

func listHandler(*gofr.Context) (any, error) {
	// Get data from somewhere
	data := TodoPageData{
		PageTitle: "My TODO list",
		Todos: []Todo{
			{Title: "Expand on Gofr documentation ", Done: false},
			{Title: "Add more examples", Done: true},
			{Title: "Write some articles", Done: false},
		},
	}

	return response.Template{Data: data, Name: "todo.html"}, nil
}
