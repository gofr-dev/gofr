# Overriding Default

GoFr allows overriding default behavior of its features.

## Raw response format

GoFr by default wraps a handler's return value and assigns it to the `data` field in a response.

### Example

```go
package main

import "gofr.dev/pkg/gofr"

type user struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := gofr.New()

	app.GET("/users", func(ctx *gofr.Context) (any, error) {
		users := []user{{ID: 1, Name: "Daria"}, {ID: 2, Name: "Ihor"}}

		return users, nil
	})

	app.Run()
}
```

Response example:
```json
{
  "data": [
    {
      "id": 1,
      "name": "Daria"
    },
    {
      "id": 2,
      "name": "Ihor"
    }
  ]
}
```

If you want to have a raw response structure - wrap it in `response.Raw`:
```go
app.GET("/users", func(ctx *gofr.Context) (any, error) {

    users := []user{{ID: 1, Name: "Daria"}, {ID: 2, Name: "Ihor"}}

    return response.Raw{Data: users}, nil
})
```

Response example:
```json
[
  {
    "id": 1,
    "name": "Daria"
  },
  {
    "id": 2,
    "name": "Ihor"
  }
]
```

## Rendering Templates
GoFr makes it easy to render HTML and HTMX templates directly from your handlers using the response.Template type.
By convention, all template files—whether HTML or HTMX—should be placed inside a templates directory located at the root of your project.

### Example
```go
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

func listHandler(ctx *gofr.Context) (any, error) {
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
```

## HTTP Redirects

GoFr allows redirecting HTTP requests to other URLs using the `response.Redirect` type.

### Example

```go
package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/pkg/gofr/http/response"
)

func main() {
	app := gofr.New()

	app.GET("/old-page", func(ctx *gofr.Context) (any, error) {
		// Redirect to a new URL
		return response.Redirect{URL: "https://example.com/new-page"}, nil
	})

	app.Run()
}
```

In GoFr, the following HTTP methods can be redirected, along with their corresponding status codes:

- **GET (302 Found)**: It is safe to redirect because the request remains a GET after the redirect.
- **POST (303 See Other)**: The browser converts the POST request to a GET on redirect.
- **PUT (303 See Other)**: The browser converts the PUT request to a GET on redirect.
- **PATCH (303 See Other)**: The browser converts the PATCH request to a GET on redirect.
- **DELETE (302 Found)**: This is a temporary redirect, but method handling is ambiguous, as most browsers historically convert the DELETE request into a GET.


## Favicon.ico

By default, GoFr loads its own `favicon.ico` present in root directory for an application. To override `favicon.ico` user
can place its custom icon in the **static** directory of its application.

> [!NOTE]
> The custom favicon should also be named as `favicon.ico` in the static directory of application.
