# GRPC

[gRPC](https://grpc.io/about/) Server in gofr can be initialised by adding the configuraton `GRPC_PORT`.

GoFr supports automatic tracing between gRPC and HTTP calls, making it easier than ever to monitor and trace the flow of requests as they traverse different protocols within your application.

To get started with gRPC in Gofr, you will need to create a Protobuf definition for your gRPC service. Once you have created your Protobuf definition, you can use Gofr to create gRPC server and client.

## Usage

Create `grpc.proto` file in grpc directory.

**grpc.proto**

```go
syntax = "proto3";
option go_package = "/grpc";

// The greeting service definition.
service Greeter {
    // Sends a greeting
    rpc SayHello (HelloRequest) returns (HelloReply) {}
}

// The request message containing the user's name.
message HelloRequest {
    string name = 1;
}

// The response message containing the greetings
message HelloReply {
    string message = 1;
}
```

Generate the [protobuf](https://protobuf.dev/reference/go/go-generated/) files using the following grpc commands.

```bash
protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. grpc/grpc.proto
```

# Server

To set the port on which your grpc server will start add the following configs in `.env` file in server/configs directory.

```bash
GRPC_PORT=10000
```

Add the following code in server/main.go.

```go
package main

import (
    "context"

    "gofr.dev/pkg/errors"
    "gofr.dev/pkg/gofr"

    "github.com/example/grpc"
)

func main() {
    app := gofr.New()

    // Registers the gRPC server with the Gofr server.
    grpc.RegisterGreeterServer(app.Server.GRPC.Server(), server{})

    app.Start()
}

// A gRPC server that implements the GreeterServer interface.
type server struct {
    grpc.UnimplementedGreeterServer
}

// SayHello Implements the method of the GreeterServer interface.
// takes the HelloRequest struct defined in the proto file as parameter, and HelloReply struct as the output
func (h server) SayHello(_ context.Context, name *grpc.HelloRequest) (*grpc.HelloReply, error) {
    if name.Name != "" {
        resp := &grpc.HelloReply{
            Message: "Hello " + string(name.Name),
        }

        return resp, nil
    }

    return nil, errors.MissingParam{Param: []string{"name"}}
}
```

Your .env after adding the configs will look like following

```bash
GRPC_PORT=10000
```

# Client

To set the url for grpc server add the following configs in `.env` file in `client/configs` directory.

```bash
# change http port as server is running on port 8000.
HTTP_PORT=8001
GRPC_SERVER=localhost:10000
```

We will create an http client and use it to hit the gRPC server, to get response.

Create `main.go` in `client` directory

```go
package main

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"gofr.dev/pkg/gofr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcProto "github.com/example/grpc"
)

func main() {
	app := gofr.New()

	// Establish a gRPC connection to the server defined in the app's configuration.
	conn, err := grpc.Dial(app.Config.Get("GRPC_SERVER"), grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		app.Logger.Error(err)
	}

  // Create a gRPC client using the connection.
	c := grpcProto.NewGreeterClient(conn)

  // Define a route for handling HTTP GET requests to "/hello".
	app.GET("/hello", func(ctx *gofr.Context) (interface{}, error) {
		// Perform a gRPC call to the "SayHello" method on the server.
		resp, err := c.SayHello(ctx.Context, &grpcProto.HelloRequest{Name: ctx.Param("name")})
		if err != nil {
			return nil, err
		}

		return resp, nil
	})

	app.Start()
}
```

Your .env after adding the configs will look like following

---

You will have the following directory structure.

```bash
├── client
│   ├── configs
│   └── main.go
├── go.mod
├── go.sum
├── grpc
│   ├── grpc.pb.go
│   ├── grpc.proto
│   └── grpc_grpc.pb.go
└── server
    ├── configs
    └── main.go
```
