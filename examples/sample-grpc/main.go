package main

import (
	"gofr.dev/examples/sample-grpc/handler/grpc"
	"gofr.dev/examples/sample-grpc/handler/http"
	"gofr.dev/pkg/gofr"
)

func main() {
	// this example shows an applicationZ that uses both, HTTP and GRPC
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	app.GET("/example", http.GetUserDetails)

	grpcHandler := grpc.New()

	grpc.RegisterExampleServiceServer(app.Server.GRPC.Server(), grpcHandler)

	app.Start()
}
