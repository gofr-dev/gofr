# Prerequisite
-  Go 1.20 or above.
   To check the version use the following command `go version`.

-  Prior familiarity with Golang syntax is essential. {% new-tab-link title="Golang Tour" href="https://tour.golang.org/" /%} is highly recommended as it has an excellent guided tour.
   
## Write your first GoFr API

Let's start by initializing the go module by using the following command.

```bash
go mod init github.com/example
```

To know more about go modules refer {% new-tab-link title="here" href="https://go.dev/ref/mod" /%}.

Add {% new-tab-link title="gofr" href="https://github.com/gofr-dev/gofr" /%} package to the project using the following command

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

    // Runs the server, it will listen on the default port 8000.
    // it can be over-ridden through configs
   app.Run()
}
```

Before running the server run the following go command to download and sync the required modules.

`go mod tidy`

To run the server, use the command

`go run main.go`

This would start the server at 8000 port, you can access {% new-tab-link title="http://localhost:8000/greet" href="http://localhost:8000/greet" /%} from your browser, you would be able to see the output as following with _Status Code 200_ as per REST Standard

```json
{ "data": "Hello World!" }
```

## Understanding the example

The `hello-world` server involves three essential steps:

1. **Creating GoFr Server:**

   When `gofr.New()` is called, it initializes the framework and handles various setup tasks like initialising logger, metrics, datasources etc based on the configs.

   _This single line is a standard part of all gofr-based servers._


2. **Attaching a Handler to a Path:**

   In this step, we instruct the server to associate an HTTP request with a specific handler function. This is achieved through `app.GET("/greet", HandlerFunction)`, where _GET /greet_ maps to HandlerFunction. Likewise, `app.POST("/todo", ToDoCreationHandler)` links a _POST_ request to the /todo endpoint with _ToDoCreationHandler_.


   **Good To Know**

>  In Go, functions are first-class citizens, allowing easy handler definition and reference.
   HTTP Handler functions should follow the `func(ctx *gofr.Context) (interface{}, error)` signature.
   They take a context as input, returning two values: the response data and an error (set to `nil` when there is no error).

   In GoFr `ctx *gofr.Context` serves as a wrapper for requests, responses, and dependencies, providing various functionalities.

   For more details about context, refer {% new-tab-link title="here" href="/docs/references/context" /%}.

3. **Starting the server**

   When `app.Run()` is called, it configures ,initiates and runs the HTTP server, middlewares. It manages essential features such as routes for health check endpoints, metrics server, favicon etc. It starts the server on the default port 8000.
