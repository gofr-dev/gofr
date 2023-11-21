## Prerequisite

- Install or update [Go](https://go.dev/dl/).

  To check the version use the following command `go version`.

- Prior familiarity with Golang syntax is essential. [Golang Tour](https://tour.golang.org/) is highly recommended as it has an excellent guided tour.

## Write your first GoFr API

Let's start by initializing the go module by using the following command.

```bash
go mod init github.com/example
```

To know more about go modules refer [here](https://go.dev/ref/mod)

Add [gofr](https://github.com/gofr-dev/gofr) package to the project using the following command

```bash
go get gofr.dev
```

Now add the following code to _main.go_ file

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    // initialise gofr object
    app := gofr.New()

    // register route greet
    app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {

        return "Hello World!", nil
    })

    // Starts the server, it will listen on the default port 8000.
    // it can be over-ridden through configs
    app.Start()
}
```

Before running the server run the following command to go needs to download and sync the required modules.

`go mod tidy`

To run the server, use the command

`go run main.go`

This would start the server at 8000 port, you can access [http://localhost:8000/greet](http://localhost:8000/greet) from your browser, you would be able to see the output as following with _Status Code 200_ as per REST Standard

```json
{ "data": "Hello World!" }
```

## Understanding the example

The `hello-world` server involves three essential steps:

1. **Creating GoFr Server:**

   When `gofr.New()` is called, it initializes the framework and handles various setup tasks like configuring database connection pools, initialising logger, setting CORS headers etc based on the configs.

   _This single line is a standard part of all gofr-based servers._

2. **Attaching a Handler to a Path:**

   In this step, we instruct the server to associate an HTTP request with a specific handler function. This is achieved through `app.GET("/greet", HandlerFunction)`, where _GET /hello_ maps to HandlerFunction. Likewise, `app.POST("/todo", ToDoCreationHandler)` links a _POST_ request to the /todo endpoint with _ToDoCreationHandler_.

   **Good To Know**

   In Go, functions are first-class citizens, allowing easy handler definition and reference.
   Handler functions should follow the `func(ctx *gofr.Context) (interface{}, error)` signature.
   They take a context as input, returning two values: the response data and an error (set to `nil` on success).

   In GoFr `ctx *gofr.Context` serves as a wrapper for requests, responses, and dependencies, providing various functionalities.

   For more details about context, refer [here](/docs/v1/references/context).

3. **Starting the server**

   When `app.Start()` is called, it configures and initiates the HTTP server, middlewares based on provided configs. It manages essential features such as routes for health checks, swagger UI etc. It starts the server on the default port 8000.
