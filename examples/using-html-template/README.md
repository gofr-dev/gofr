# Using HTML Template

This GoFr example demonstrates the use of html template, GoFr supports both static and dynamic html templates.
All template files—whether HTML or HTMX—should be placed inside a templates directory located at the root of your project.


### Usage
```go
// path to the static html files
app.AddStaticFiles("/", "./static")

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
    // provide data and template name to response.Template 
	return response.Template{Data: data, Name: "todo.html"}, nil
}
```

### To run the example use the command below:
```console
go run main.go
```
