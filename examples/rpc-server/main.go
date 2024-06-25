package main

import (
	"fmt"
	"gofr.dev/pkg/gofr"
)

// ExampleRequest defines the structure of the request.
type ExampleRequest struct {
	ID   int
	Name string
}

// ExampleResponse defines the structure of the response.
type ExampleResponse struct {
	Status  string
	Message string
}

// ExampleService is an interface defining the method(s) to be exposed over RPC.
type ExampleService interface {
	GetExampleData(ctx *gofr.Context, req ExampleRequest, resp *ExampleResponse) error
}

// ExampleServiceImpl is the implementation of ExampleService.
type ExampleServiceImpl struct {
}

// GetExampleData implements the ExampleService interface method.
func (e *ExampleServiceImpl) GetExampleData(ctx *gofr.Context, req ExampleRequest, resp *ExampleResponse) error {
	ctx.Log("I am here hahaha")

	resp.Status = "hey"
	resp.Message = fmt.Sprintf("Received ID: %d, Name: %s", req.ID, req.Name)

	return nil
}

func main() {
	// Register ExampleService implementation with RPC server
	exampleService := &ExampleServiceImpl{}

	app := gofr.New()

	app.RegisterRPCService(exampleService)

	app.Run()
}
