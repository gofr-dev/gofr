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

func main() {
	app := gofr.New()

	app.RegisterRPCClient("example", "localhost:9033")

	// Get the RPC client
	rpcClient := app.GetRPCClient("example")

	// Example request
	request := ExampleRequest{
		ID:   1,
		Name: "abc Doe",
	}

	// Prepare to receive response
	var response ExampleResponse

	// Call the RPC method GetExampleData on the server
	err := rpcClient.Call("ExampleServiceImpl.GetExampleData", request, &response)
	if err != nil {
		fmt.Println("Error calling RPC method:", err)
		return
	}

	// Print the response received from the server
	fmt.Printf("Response: %+v\n", response)

	app.Run()
}
