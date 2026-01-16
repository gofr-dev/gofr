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

service <SERVICE_NAME>Service {
  rpc <SERVICE_METHOD> (<SERVICE_REQUEST>) returns (<SERVICE_RESPONSE>) {}
        }
```

**2. Specify Request and Response Types:**

Users must define the type of message being exchanged between server and client, for protocol buffer to serialize them when making a remote
procedure call. Below is a generic representation for services' gRPC messages type.

```protobuf
message <SERVICE_REQUEST> {
    int64 id = 1;
    string name = 2;
    // other fields that can be passed
        }

message <SERVICE_RESPONSE> {
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
	<SERVICE_NAME>.proto
```

This command generates two files, `<SERVICE_NAME>.pb.go` and `<SERVICE_NAME>_grpc.pb.go`, containing the necessary code for performing RPC calls.

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

This command leverages the `gofr-cli` to generate a `<SERVICE_NAME>_server.go` file (e.g., `customer_server.go`)
containing a template for your gRPC server implementation, including context support, in the same directory as
that of the specified proto file.

**2. Modify the Generated Code:**

- Customize the `<SERVICE_NAME>GoFrServer` struct with required dependencies and fields.
- Implement the `<SERVICE_METHOD>` method to handle incoming requests, as required in this usecase:
  - Bind the request payload using `ctx.Bind(&<SERVICE_REQUEST>)`.
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

    packageName.Register<SERVICE_NAME>ServerWithGofr(app, &<PACKAGE_NAME>.New<SERVICE_NAME>GoFrServer())

    app.Run()
}
```

>Note: By default, gRPC server will run on port 9000, to customize the port users can set `GRPC_PORT` config in the .env

## Adding gRPC Server Options

To customize your gRPC server, use `AddGRPCServerOptions()`.

### Example: Enabling TLS & other ServerOptions
```go
func main() {
    app := gofr.New()

    // Add TLS credentials and connection timeout in one call
    creds, _ := credentials.NewServerTLSFromFile("server-cert.pem", "server-key.pem")
	
    app.AddGRPCServerOptions(
		grpc.Creds(creds),
    	grpc.ConnectionTimeout(10 * time.Second),
    )

    packageName.Register<SERVICE_NAME>ServerWithGofr(app, &<PACKAGE_NAME>.New<SERVICE_NAME>GoFrServer())

    app.Run()
}
```

## Adding Custom Unary Interceptors

Interceptors help in implementing authentication, validation, request transformation, and error handling.

### Example: Authentication Interceptor
```go
func main() {
    app := gofr.New()

    app.AddGRPCUnaryInterceptors(authInterceptor)

    packageName.Register<SERVICE_NAME>ServerWithGofr(app, &<PACKAGE_NAME>.New<SERVICE_NAME>GoFrServer())

    app.Run()
}

func authInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    if !isAuthenticated(ctx) {
        return nil, status.Errorf(codes.Unauthenticated, "authentication failed")
    }

    return handler(ctx, req)
}
```

### Built-in Authentication Interceptors

GoFr provides built-in interceptors for common authentication methods, ensuring parity with HTTP services. These can be found in `gofr.dev/pkg/gofr/grpc/middleware`.

*   **Basic Auth**: `BasicAuthUnaryInterceptor` and `BasicAuthStreamInterceptor`
*   **API Key**: `APIKeyAuthUnaryInterceptor` and `APIKeyAuthStreamInterceptor`
*   **OAuth (JWT)**: `OAuthUnaryInterceptor` and `OAuthStreamInterceptor`

For detailed usage, refer to the [gRPC Authentication documentation](https://gofr.dev/docs/advanced-guide/grpc-authentication).

## Built-in Observability

GoFr provides seamless observability for gRPC services, including standardized tracing, metrics, and logging.

### Standardized OTEL Tracing

GoFr's gRPC interceptors are fully compliant with OpenTelemetry (OTEL) standards. They automatically:
1.  **Extract Context**: Use standard OTEL propagators to extract trace context from incoming metadata.
2.  **Propagate Context**: Ensure trace information is passed down to your handlers via `gofr.Context`.
3.  **Backward Compatibility**: Support GoFr's legacy headers (`x-gofr-traceid`, `x-gofr-spanid`) if standard OTEL headers are missing.

This standardization allows GoFr services to integrate effortlessly with any OTEL-compatible monitoring tools like Jaeger, Zipkin, or SigNoz.

## Generating gRPC Client using `gofr wrap grpc client`

**1. Use the `gofr wrap grpc client` Command:**
   ```bash
gofr wrap grpc client -proto=./path/your/proto/file
   ```
This command leverages the `gofr-cli` to generate a `<SERVICE_NAME>_client.go` file (e.g., `customer_client.go`). This file must not be modified.

**2. Register the connection to your gRPC service inside your <SERVICE_METHOD> and make inter-service calls as follows :**

   ```go
// gRPC Handler with context support
func <SERVICE_METHOD>(ctx *gofr.Context) (*<SERVICE_RESPONSE>, error) {
	// Create the gRPC client
    srv, err := New<SERVICE_NAME>GoFrClient("your-grpc-server-host", ctx.Metrics())
    if err != nil {
        return nil, err
    }

    // Prepare the request
    req := &<SERVICE_REQUEST>{
    // populate fields as necessary
    }

	// Call the gRPC method with tracing/metrics enabled
    res, err := srv.<SERVICE_METHOD>(ctx, req)
    if err != nil {
        return nil, err
    }

    return res, nil
}
```
## Error Handling and Validation
GoFr's gRPC implementation includes built-in error handling and validation:

**Port Validation**: Automatically validates that gRPC ports are within valid range (1-65535)
**Port Availability**: Checks if the specified port is available before starting the server
**Server Creation**: Validates server creation and provides detailed error messages
**Container Injection**: Validates container injection into gRPC services with detailed logging

Port Configuration
```bash
// Set custom gRPC port in .env file
GRPC_PORT=9001

// Or use default port 9000 if not specified
```
## gRPC Reflection
GoFr supports gRPC reflection for easier debugging and testing. Enable it using the configuration:
```bash
# In your .env file
GRPC_ENABLE_REFLECTION=true
```
When enabled, you can use tools like grpcurl to inspect and test your gRPC services:

```bash
# List available services
grpcurl -plaintext localhost:9000 list

# Describe a service
grpcurl -plaintext localhost:9000 describe YourService

# Make a test call
grpcurl -plaintext -d '{"name": "test"}' localhost:9000 YourService/YourMethod
```

## Built-in Metrics
GoFr automatically registers the following gRPC metrics:

+ **grpc_server_status**: Gauge indicating server status (1=running, 0=stopped)
+ **grpc_server_errors_total**: Counter for total gRPC server errors
+ **grpc_services_registered_total**: Counter for total registered gRPC services

These metrics are automatically available in your metrics endpoint and can be used for monitoring and alerting.

## Customizing gRPC Client with DialOptions

GoFr provides flexibility to customize your gRPC client connections using gRPC `DialOptions`. This allows users to configure aspects such as transport security, interceptors, and load balancing policies.
You can pass optional parameters while creating your gRPC client to tailor the connection to your needs. Here’s an example of a Unary Interceptor that sets metadata on outgoing requests:

```go
func main() {
    app := gofr.New()

    // Create a gRPC client for the service
    gRPCClient, err := client.New<SERVICE_NAME>GoFrClient(
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

// MetadataUnaryInterceptor sets a custom metadata value on outgoing requests
func MetadataUnaryInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    md := metadata.Pairs("client-id", "GoFr-Client-123")
    ctx = metadata.NewOutgoingContext(ctx, md)

    err := invoker(ctx, method, req, reply, cc, opts...)
    if err != nil {
        return fmt.Errorf("Error in %s: %v", method, err)
	}
	
	return err
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

    gRPCClient, err := client.New<SERVICE_NAME>GoFrClient(
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
type <SERVICE_NAME>GoFrClient interface {
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
type <SERVICE_NAME>GoFrServer struct {
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