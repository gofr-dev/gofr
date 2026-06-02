---
description: "Generate gRPC server and client wrappers with built-in tracing, metrics, and logging using the gofr wrap grpc command."
nextjs:
  metadata:
    title: "gofr wrap grpc — Generate gRPC Server/Client Wrappers"
    description: "Generate gRPC server and client wrappers with built-in tracing, metrics, and logging using the gofr wrap grpc command."
---

# gofr wrap grpc

* The gofr wrap grpc command streamlines gRPC integration in a GoFr project by generating GoFr's context-aware structures.
* It simplifies setting up gRPC handlers with minimal steps, and accessing datasources, adding tracing as well as custom metrics. Based on the proto file it creates the handler/client with GoFr's context.
  For detailed instructions on using grpc with GoFr see the [gRPC documentation](/docs/advanced-guide/grpc)

## Command Usage
**gRPC Server**
```bash
  gofr wrap grpc server --proto=<path_to_the_proto_file>
```
## Generated Files
**Server**
- ```{serviceName}_gofr.go (auto-generated; do not modify)```
- ```{serviceName}_server.go (example structure below)```

## Example Usage
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


## Example Usage:
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

---

## See also

- [GoFr CLI overview](/docs/references/gofrcli)
- [`gofr init`](/docs/references/gofrcli/init)
- [`gofr migrate`](/docs/references/gofrcli/migrate)
- [`gofr store`](/docs/references/gofrcli/store)
