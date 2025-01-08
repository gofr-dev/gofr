# gRPC with Gofr

This guide explains how to leverage Gofr to streamline the development of gRPC handlers in Go. Gofr empowers you to create gRPC handlers efficiently while taking advantage of context support for effective dependency and tracing management within your handlers.

## Prerequisites

**1. Protocol Buffer Compiler (`protoc`) Installation:**

**Linux (using `apt` or `apt-get`):**

```bash
  sudo apt install -y protobuf-compiler
  protoc --version  # Ensure compiler version is 3+
```

**macOS (using Homebrew):**

```bash
  brew install protobuf
  protoc --version  # Ensure compiler version is 3+
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

For a detailed guide, refer to the official gRPC documentation's tutorial: https://grpc.io/docs/languages/go/basics/

**1. Define Your Service and RPC Methods:**

Create a `.proto` file (e.g., `customer.proto`) to define your service and the RPC methods it provides:

   ```protobuf
   // Indicates the protocol buffer version that is being used
   syntax = "proto3";
   // Indicates the go package where the generated file will be produced
   option go_package = "";

   service CustomerService {
       rpc GetCustomer (CustomerFilter) returns (CustomerData) {}
   }
   ```

**2. Specify Request and Response Types:**

For example: The CustomerFilter and CustomerData are two types of messages that will be exchanged between server and client. Users must define those for protocol buffer to serialize them when making a remote procedure call.

```go
message CustomerFilter {
int64 id = 1;
string name = 2;
// other fields that can be passed
}

message CustomerData {
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
   customer.proto
   ```

This command generates two files, `customer.pb.go` and `customer_grpc.pb.go`, containing the necessary code for performing RPC calls.

## Generating gRPC Handler Template using `gofr wrap grpc` (Recommended)

#### Prerequisite: gofr-cli must be installed

To install the CLI -
```bash
  go install gofr.dev/cli/gofr@latest
```

To check the installation -
```bash
  gofr version
```


**1. Use the `gofr wrap grpc` Command:**

   ```bash
     gofr wrap grpc -proto=./customer.proto
   ```

This command leverages the `gofr-cli` to generate a `{serviceName}Server.go` file (e.g., `CustomerServer.go`) containing a template for your gRPC server implementation, including context support.

**2. Modify the Generated Code:**

- Customize the `CustomerServer` struct with required dependencies and fields.
- Implement the `GetCustomer` method to handle incoming requests:
    - Bind the request payload using `ctx.Bind(&request)`.
    - Process the request and generate a response.

## Registering the gRPC Service with Gofr

**1. Import Necessary Packages:**

   ```go
   import (
       "gofr.dev/pkg/gofr"
       "gofr.dev/examples/grpc-server/customer"
   )
   ```

**2. Register the Service in Your `main.go`:**

   ```go
   func main() {
       app := gofr.New()

       customer.RegisterCustomerServiceServer(app, &customer.CustomerServer{})

       app.Run()
   }
   ```

## Alternative Approach without `gofr wrap grpc` (Limited Functionality)

**1. Implement the `CustomerServiceServer` Interface:**

In `customer.pb.go`, you'll find the `CustomerServiceServer` interface:

   ```go
   type CustomerServiceServer interface {
       GetCustomer(context.Context, *CustomerFilter) (*CustomerData, error)
   }
   ```

**2. User needs to implement this interface to serve the content to the client calling the method.**

```go
package customer

import (
"context"
)

type Handler struct {
// required fields to get the customer data
}

func (h *Handler) GetCustomer(ctx context.Context, filter *CustomerFilter) (*CustomerData, error) {
// get the customer data and handler error
return data, nil
}
   ```
**3. Lastly to register the gRPC service to the GoFr server, user can call the RegisterCustomerServiceServer in customer_grpc.pb.go to register the service giving GoFr app and the Handler struct.**

```go
package main

import (
"gofr.dev/pkg/gofr"
"gofr.dev/examples/grpc-server/customer"
)

func main() {
    app := gofr.New()

	customer.RegisterCustomerServiceServer(app, customer.Handler{})

	app.Run()
}
```

> Note: By default, gRPC server will run on port 9000, to customize the port users can set GRPC_PORT config in the .env

> #### Check out the example of setting up a gRPC server in GoFr: Visit GitHub