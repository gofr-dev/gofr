# gRPC
We have already seen how GoFr can help ease the development of HTTP servers, but there are
cases where performance is primarily required sacrificing flexibility. In these types of 
scenarios gRPC protocol comes into picture. {% new-tab-link title="gRPC" href="https://grpc.io/docs/what-is-grpc/introduction/" /%} is an open-source RPC(Remote Procedure Call)
framework initially developed by Google. 

## Prerequisites
- Install the `protoc` protocol buffer compilation
    - Linux, using `apt` or `apt-get`
        ```shell
        $ apt install -y protobuf-compiler
        $ protoc --version  # Ensure compiler version is 3+
        ```
    - macOS, using Homebrew
        ```shell
        $ brew install protobuf
        $ protoc --version  # Ensure compiler version is 3+  
        ```
- Install **Go Plugins** for protocol compiler:
    1. Install protocol compiler plugins for Go
       ```shell
       $ go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
       $ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
       ```
    2. Update `PATH` for `protoc` compiler to find the plugins:
       ```shell
       $ export PATH="$PATH:$(go env GOPATH)/bin"
       ```
       
## Creating protocol buffers
For a detailed guide, please take a look at the {% new-tab-link title="Tutorial" href="https://grpc.io/docs/languages/go/basics/" /%} at official gRPC docs.

We need to create a `customer.proto` file to define our service and the RPC methods that the service provides.
```protobuf
// Indicates the protocol buffer version that is being used
syntax = "proto3";
// Indicates the go package where the generated file will be produced
option go_package = "";

service CustomerService {
  // ...
}
```
Inside the service one can define all the `rpc` methods, specifying the request and responses types.
```protobuf
service CustomerService {
  // GetCustomer is a rpc method to get customer data using specific filters
  rpc GetCustomer(CustomerFilter) returns(CustomerData) {}
}
```
The `CustomerFilter` and `CustomerData` are two types of messages that will be exchanged between server
and client. Users must define those for protocol buffer to serialize them when making a remote procedure call.
```protobuf
syntax = "proto3";

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

Now run the following command to generate go code using the Go gRPC plugins:
```shell
protoc \
--go_out=. \
--go_opt=paths=source_relative \
--go-grpc_out=. \
--go-grpc_opt=paths=source_relative \ 
customer.proto
```
Above command will generate two files `customer.pb.go` and `customer_grpc.pb.go` and these contain necessary code to perform RPC calls.
In `customer.pb.go` you can find `CustomerService` interface-
```go
// CustomerServiceServer is the server API for CustomerService service.
type CustomerServiceServer interface {
    GetCustomer(context.Context, *CustomerFilter) (*CustomerData, error)
}
```
User needs to implement this interface to serve the content to the client calling the method.
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

Lastly to register the gRPC service to the GoFr server, user can call the `RegisterCustomerServiceServer` in `customer_grpc.pb.go`
to register the service giving GoFr app and the Handler struct.
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
>Note: By default, gRPC server will run on port 9000, to customize the port users can set `GRPC_PORT` config in the .env