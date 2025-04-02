# gRPC with Gofr

We have already seen how GoFr can help ease the development of HTTP servers, but there are cases where performance is primarily required sacrificing flexibility. In these types of scenarios gRPC protocol comes into picture. {% new-tab-link title="gRPC" href="https://grpc.io/docs/what-is-grpc/introduction/" /%} is an open-source RPC(Remote Procedure Call) framework initially developed by Google.

GoFr streamlines the creation of gRPC servers and clients with unified GoFr's context support. 
It provides built-in tracing, metrics, and logging to ensure seamless performance monitoring for both gRPC servers and inter-service gRPC communication. 
With GoFr's context, you can seamlessly define custom metrics and traces across gRPC handlers, ensuring consistent observability and streamlined debugging throughout 
your system. Additionally, GoFr provides a built-in health check for all your services and supports inter-service 
health checks, allowing gRPC services to monitor each other effortlessly.

## Prerequisites

**1. Protocol Buffer Compiler (`protoc`) Installation:**

- **Linux (using `apt` or `apt-get`):**

```bash
sudo apt install -y protobuf-compiler
protoc --version # Ensure compiler version is 3+
```

- **macOS (using Homebrew):**

```bash
brew install protobuf
protoc --version # Ensure compiler version is 3+
```

**2. Go Plugins for Protocol Compiler:**

a. Install protocol compiler plugins for Go:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
```

b. Update `PATH` for `protoc` to locate the plugins:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Creating Protocol Buffers

For a detailed guide, refer to the official gRPC documentation's tutorial: {% new-tab-link title="Tutorial" href="https://grpc.io/docs/languages/go/basics/" /%} at official gRPC docs.

**1. Define Your Service and RPC Methods:**

Create a `.proto` file (e.g., `customer.proto`) to define your service and the RPC methods it provides:

```protobuf
// Indicates the protocol buffer version that is being used
syntax = "proto3";
// Indicates the go package where the generated file will be produced
option go_package = "path/to/your/proto/file";

service {serviceName}Service {
rpc {serviceMethod} ({serviceRequest}) returns ({serviceResponse}) {}
        }
```

**2. Specify Request and Response Types:**

Users must define the type of message being exchanged between server and client, for protocol buffer to serialize them when making a remote
procedure call. Below is a generic representation for services' gRPC messages type.

```protobuf
message {serviceRequest} {
int64 id = 1;
        string name = 2;
// other fields that can be passed
        }

message {serviceResponse} {
int64 id = 1;
        string name = 2;
string address = 3;
// other customer related fields
        }
```

**3. Generate Go Code:**

Run the following command to generate Go code using the Go gRPC plugins:

```bash
protoc \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	{serviceName}.proto
```

This command generates two files, `{serviceName}.pb.go` and `{serviceName}_grpc.pb.go`, containing the necessary code for performing RPC calls.

## Prerequisite: gofr-cli must be installed
To install the CLI -

```bash
go install gofr.dev/cli/gofr@latest
```

## Generating gRPC Server Handler Template using `gofr wrap grpc server`

**1. Use the `gofr wrap grpc server` Command:**
   ```bash
gofr wrap grpc server -proto=./path/your/proto/file
   ```

This command leverages the `gofr-cli` to generate a `{serviceName}_server.go` file (e.g., `customer_server.go`)
containing a template for your gRPC server implementation, including context support, in the same directory as
that of the specified proto file.

**2. Modify the Generated Code:**

- Customize the `{serviceName}GoFrServer` struct with required dependencies and fields.
- Implement the `{serviceMethod}` method to handle incoming requests, as required in this usecase:
  - Bind the request payload using `ctx.Bind(&{serviceRequest})`.
  - Process the request and generate a response.

## Registering the gRPC Service with Gofr

**1. Import Necessary Packages:**

```go
import (
	"path/to/your/generated-grpc-server/packageName"

	"gofr.dev/pkg/gofr"
)
```

**2. Register the Service in your `main.go`:**

```go
func main() {
    app := gofr.New()

    packageName.Register{serviceName}ServerWithGofr(app, &{packageName}.New{serviceName}GoFrServer())

    app.Run()
}
```

>Note: By default, gRPC server will run on port 9000, to customize the port users can set `GRPC_PORT` config in the .env

## Generating gRPC Client using `gofr wrap grpc client`

**1. Use the `gofr wrap grpc client` Command:**
   ```bash
gofr wrap grpc client -proto=./path/your/proto/file
   ```
This command leverages the `gofr-cli` to generate a `{serviceName}_client.go` file (e.g., `customer_client.go`). This file must not be modified.

**2. Register the connection to your gRPC service inside your {serviceMethod} and make inter-service calls as follows :**

   ```go
// gRPC Handler with context support
func {serviceMethod}(ctx *gofr.Context) (*{serviceResponse}, error) {
// Create the gRPC client
srv, err := New{serviceName}GoFrClient("your-grpc-server-host", ctx.Metrics())
if err != nil {
return nil, err
}

// Prepare the request
req := &{serviceRequest}{
// populate fields as necessary
}

// Call the gRPC method with tracing/metrics enabled
res, err := srv.{serviceMethod}(ctx, req)
if err != nil {
return nil, err
}

return res, nil
}
```

## Customizing gRPC Client with DialOptions

GoFr provides flexibility to customize your gRPC client connections using gRPC DialOptions. This allows users to configure aspects such as transport security, interceptors, and load balancing policies.
You can pass optional parameters while creating your gRPC client to tailor the connection to your needs. Here’s an example of a Unary Interceptor that sets metadata on outgoing requests:

```go
// MetadataUnaryInterceptor sets a custom metadata value on outgoing requests
func MetadataUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    md := metadata.Pairs("client-id", "GoFr-Client-123")
    ctx = metadata.NewOutgoingContext(ctx, md)

    err := invoker(ctx, method, req, reply, cc, opts...)
    if err != nil {
        return fmt.Errorf("Error in %s: %v", method, err)
    }
	
    return err
}

func main() {
    app := gofr.New()

    // Create a gRPC client for the service
    gRPCClient, err := client.New{serviceName}GoFrClient(
        app.Config.Get("GRPC_SERVER_HOST"),
        app.Metrics(),
        grpc.WithChainUnaryInterceptor(MetadataUnaryInterceptor),
    )
    if err != nil {
        app.Logger().Errorf("Failed to create gRPC client: %v", err)
        return
    }

    greet := NewGreetHandler(gRPCClient)

    app.GET("/hello", greet.Hello)

    app.Run()
}
```

This interceptor sets a metadata key `client-id` with a value of `GoFr-Client-123` for each request. Metadata can be used for authentication, tracing, or custom behaviors.

### Using TLS Credentials and Advanced Service Config
By default, gRPC connections in GoFr are made over insecure connections, which is not recommended for production. You can override this behavior using TLS credentials. Additionally, a more comprehensive service configuration can define retry policies and other settings:

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

// The default serviceConfig in GoFr only sets the loadBalancingPolicy to "round_robin".
const serviceConfig = `{
    "loadBalancingPolicy": "round_robin", 
    "methodConfig": [{
        "name": [{"service": "HelloService"}],
        "retryPolicy": {
            "maxAttempts": 4,
            "initialBackoff": "0.1s",
            "maxBackoff": "1s",
            "backoffMultiplier": 2.0,
            "retryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
        }
    }]
}`

func main() {
    app := gofr.New()

    creds, err := credentials.NewClientTLSFromFile("path/to/cert.pem", "")
    if err != nil {
        app.Logger().Errorf("Failed to load TLS certificate: %v", err)
        return
    }

    gRPCClient, err := client.New{serviceName}GoFrClient(
        app.Config.Get("GRPC_SERVER_HOST"),
        app.Metrics(),
        grpc.WithTransportCredentials(creds),
        grpc.WithDefaultServiceConfig(serviceConfig),
    )
    if err != nil {
        app.Logger().Errorf("Failed to create gRPC client: %v", err)
        return
    }

    greet := NewGreetHandler(gRPCClient)

    app.GET("/hello", greet.Hello)

    app.Run()
}
```

In this example:
- `WithTransportCredentials` sets up TLS security.
- `WithDefaultServiceConfig` defines retry policies with exponential backoff and specific retryable status codes.

### Further Reading
For more details on configurable DialOptions, refer to the [official gRPC package for Go](https://pkg.go.dev/google.golang.org/grpc#DialOption).

## HealthChecks in GoFr's gRPC Service/Clients
Health Checks in GoFr's gRPC Services

GoFr provides built-in health checks for gRPC services, enabling observability, monitoring, and inter-service health verification.

### Client Interface

```go
type {serviceName}GoFrClient interface {
SayHello(*gofr.Context, *HelloRequest, ...grpc.CallOption) (*HelloResponse, error)
health
}

type health interface {
Check(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error)
Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse], error)
}
```

### Server Integration
```go
type {serviceName}GoFrServer struct {
health *healthServer
}
```
Supported Methods for HealthCheck :
```go
func (h *healthServer) Check(ctx *gofr.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error)
func (h *healthServer) Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error
func (h *healthServer) SetServingStatus(ctx *gofr.Context, service string, status grpc_health_v1.HealthCheckResponse_ServingStatus)
func (h *healthServer) Shutdown(ctx *gofr.Context)
func (h *healthServer) Resume(ctx *gofr.Context)
```
> ##### Check out the example of setting up a gRPC server/client in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/tree/main/examples/grpc)