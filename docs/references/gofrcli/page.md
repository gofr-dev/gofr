# GoFR Command Line Interface

Managing repetitive tasks and maintaining consistency across large-scale applications is challenging:

**GoFr CLI provides the following:**

* All-in-one command-line tool designed specifically for GoFr applications
* Simplifies **database migrations** management
* Automates **gRPC wrapper** generation
* Enforces standard **GoFr conventions** in new projects

**Key Benefits**
- **Streamlined gRPC Integration**: Automatically generates necessary files according to your protofiles to quickly set up gRPC services in your GoFr project.
- **Context-Aware Handling**: The generated server structure includes the necessary hooks to handle requests and responses seamlessly.
- **Minimal Configuration**: Simplifies gRPC handler setup, focusing on business logic rather than the infrastructure.

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

1. ***`init`***

   The init command initializes a new GoFr project. It sets up the foundational structure for the project and generates a basic "Hello World!" program as a starting point. This allows developers to quickly dive into building their application with a ready-made structure.

### Command Usage
```bash
  gofr init
```
---

2. ***`migrate create`***

   The migrate create command generates a migration template file with pre-defined structure in your migrations directory.
   This boilerplate code helps you maintain consistent patterns when writing database schema modifications across your project.

### Command Usage
```bash
  gofr migrate create -name=<migration-name>
```

For detailed instructions on handling database migrations, see the [handling-data-migrations documentation](../../advanced-guide/handling-data-migrations/page.md)

---

3. ***`wrap grpc`***

   * The gofr wrap grpc command streamlines gRPC integration in a GoFr project by generating context-aware structures.
   * It simplifies accessing datasources, adding tracing, and setting up gRPC handlers with minimal configuration, based on the proto file it creates the handler with gofr context.
   For detailed instructions on using grpc with GoFr see the [gRPC documentation](../../advanced-guide/grpc/page.md)

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

The generated Server code can be modified like this
```go
package server

import (
"fmt"

	"gofr.dev/pkg/gofr"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// client.RegisterHelloServerWithGofr(app, &client.HelloGoFrServer{})
//
// HelloGoFrServer defines the gRPC client implementation.
// Customize the struct with required dependencies and fields as needed.
type HelloGoFrServer struct {
}

func (s *HelloGoFrServer) SayHello(ctx *gofr.Context) (any, error) {
	request := HelloRequest{}

	err := ctx.Bind(&request)
	if err != nil {
		return nil, err
	}

	name := request.Name
	if name == "" {
		name = "World"
	}

	return &HelloResponse{
		Message: fmt.Sprintf("Hello %s!", name),
	}, nil
}
```

For detailed instruction on setting up a gRPC server with GoFr see the [gRPC Server Documentation](https://gofr.dev/docs/advanced-guide/grpc#generating-g-rpc-server-handler-template-using)

**gRPC Client**
```bash
  gofr wrap grpc client --proto=<path_to_the_proto_file>
```

**Client**
- ```{serviceName}_gofr.go (auto-generated; do not modify)```
- ```{serviceName}_client.go (example structure below)```


### Example Usage:
After generating the {serviceName}_client.go file, you can register and access the gRPC service as follows:

```go
type GreetHandler struct {
	helloGRPCClient client.HelloGoFrClient
}

func NewGreetHandler(helloClient client.HelloGoFrClient) *GreetHandler {
    return &GreetHandler{
        helloGRPCClient: helloClient,
    }
}

func (g GreetHandler) Hello(ctx *gofr.Context) (interface{}, error) {
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
    helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"))
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
For examples refer [gRPC Examples](https://github.com/gofr-dev/gofr/tree/development/examples/grpc)
