package main

import "gofr.dev/pkg/gofr"

// ExampleService Define the service interface with GoFr context
type ExampleService interface {
	GetExampleData(ctx *gofr.Context, request ExampleRequest) (ExampleResponse, error)
}

// ExampleRequest Define the request struct
type ExampleRequest struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ExampleResponse Define the response struct
type ExampleResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
