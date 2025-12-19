# GoFR Command Line Interface

Managing repetitive tasks and maintaining consistency across large-scale applications is challenging!

**GoFr CLI provides the following:**

* All-in-one command-line tool designed specifically for GoFr applications
* Simplifies **database migrations** management
* Abstracts **tracing**, **metrics** and structured **logging** for GoFr's gRPC server/client
* Enforces standard **GoFr conventions** in new projects

## Prerequisites

- Go 1.22 or above. To check Go version use the following command:
```bash
  go version
```

## **Installation**
To get started with GoFr CLI, use the below commands

```bash
  go install gofr.dev/cli/gofr@latest
```

To check the installation:
```bash
  gofr version
```
---

## Usage

The CLI can be run directly from the terminal after installation. Here’s the general syntax:

```bash
  gofr <subcommand> [flags]=[arguments]
```
---

## **Commands**

## 1. ***`init`***

   The init command initializes a new GoFr project. It sets up the foundational structure for the project and generates a basic "Hello World!" program as a starting point. This allows developers to quickly dive into building their application with a ready-made structure.

### Command Usage
```bash
  gofr init
```
---

## 2. ***`migrate create`***

   The migrate create command generates a migration template file with pre-defined structure in your migrations directory.
   This boilerplate code helps you maintain consistent patterns when writing database schema modifications across your project.


### Command Usage
```bash
  gofr migrate create -name=<migration-name>
```

### Example Usage

```bash
gofr migrate create -name=create_employee_table
```
This command generates a migration directory which has the below files:

1. A new migration file with timestamp prefix (e.g., `20250127152047_create_employee_table.go`) containing:
```go
package migrations

import (
    "gofr.dev/pkg/gofr/migration"
)

func create_employee_table() migration.Migrate {
    return migration.Migrate{
        UP: func(d migration.Datasource) error {
            // write your migrations here
            return nil
        },
    }
}
```
2. An auto-generated all.go file that maintains a registry of all migrations:
```go
// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
    "gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
    return map[int64]migration.Migrate {
        20250127152047: create_employee_table(),
    }
}
```

> **💡 Best Practice:** Learn about [organizing migrations by feature](https://gofr.dev/docs/advanced-guide/handling-data-migrations#organizing-migrations-by-feature) to avoid creating one migration per table or operation.

For detailed instructions on handling database migrations, see the [handling-data-migrations documentation](https://gofr.dev/docs/advanced-guide/handling-data-migrations)
For more examples, see the [using-migrations](https://github.com/gofr-dev/gofr/tree/main/examples/using-migrations)
---

## 3. ***`wrap grpc`***

   * The gofr wrap grpc command streamlines gRPC integration in a GoFr project by generating GoFr's context-aware structures.
   * It simplifies setting up gRPC handlers with minimal steps, and accessing datasources, adding tracing as well as custom metrics. Based on the proto file it creates the handler/client with GoFr's context.
   For detailed instructions on using grpc with GoFr see the [gRPC documentation](https://gofr.dev/docs/advanced-guide/grpc)

### Command Usage
**gRPC Server**
```bash
  gofr wrap grpc server --proto=<path_to_the_proto_file>
```
### Generated Files
**Server**
- ```{serviceName}_gofr.go (auto-generated; do not modify)```
- ```{serviceName}_server.go (example structure below)```

### Example Usage
**gRPC Server**

The command generates a server implementation template similar to this:
```go
package server

import (
   "gofr.dev/pkg/gofr"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// service.Register{ServiceName}ServerWithGofr(app, &server.{ServiceName}Server{})
//
// {ServiceName}Server defines the gRPC server implementation.
// Customize the struct with required dependencies and fields as needed.
type {ServiceName}Server struct {
}

// Example method (actual methods will depend on your proto file)
func (s *MyServiceServer) MethodName(ctx *gofr.Context) (any, error) {
   // Replace with actual logic if needed
   return &ServiceResponse{
   }, nil
}
```
For detailed instruction on setting up a gRPC server with GoFr see the [gRPC Server Documentation](https://gofr.dev/docs/advanced-guide/grpc#generating-g-rpc-server-handler-template-using)

**gRPC Client**
```bash
  gofr wrap grpc client --proto=<path_to_the_proto_file>
```

**Client**
- ```{serviceName}_client.go (example structure below)```


### Example Usage:
Assuming the service is named hello, after generating the hello_client.go file, you can seamlessly register and access the gRPC service using the following steps:

```go
type GreetHandler struct {
	helloGRPCClient client.HelloGoFrClient
}

func NewGreetHandler(helloClient client.HelloGoFrClient) *GreetHandler {
    return &GreetHandler{
        helloGRPCClient: helloClient,
    }
}

func (g GreetHandler) Hello(ctx *gofr.Context) (any, error) {
    userName := ctx.Param("name")
    helloResponse, err := g.helloGRPCClient.SayHello(ctx, &client.HelloRequest{Name: userName})
    if err != nil {
        return nil, err
    }

    return helloResponse, nil
}

func main() {
    app := gofr.New()

// Create a gRPC client for the Hello service
    helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
    if err != nil {
		app.Logger().Errorf("Failed to create Hello gRPC client: %v", err)
    return
}

    greetHandler := NewGreetHandler(helloGRPCClient)

    // Register HTTP endpoint for Hello service
    app.GET("/hello", greetHandler.Hello)

    // Run the application
    app.Run()
}
```
For detailed instruction on setting up a gRPC server with GoFr see the [gRPC Client Documentation](https://gofr.dev/docs/advanced-guide/grpc#generating-tracing-enabled-g-rpc-client-using)
For more examples refer [gRPC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/grpc)
