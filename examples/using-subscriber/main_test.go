package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	m.Run()
}

func TestMainInitialization(t *testing.T) {
	// The example registers no routes, only subscribers, so GoFr starts no HTTP server here and
	// there is no liveness endpoint to poll. The Kafka connection log is the startup signal, and
	// waiting for it is the assertion: the helper fails the test if it never arrives.
	testutil.NewServerConfigs(t)

	testutil.WaitForStdoutContains(t, "connected to 1 Kafka brokers", func() {
		go main()
	})
}

type errorRequest struct{}

func (e *errorRequest) Context() context.Context {
	return context.Background()
}

func (e *errorRequest) Bind(v interface{}) error    { return errors.New("bind error") }
func (e *errorRequest) Param(key string) string     { return "" }
func (e *errorRequest) PathParam(key string) string { return "" }
func (e *errorRequest) HostName() string            { return "" }
func (e *errorRequest) Params(key string) []string  { return nil }

func TestProductSubscribe_BindError(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)

	ctx := &gofr.Context{
		Request:       &errorRequest{},
		Container:     mockContainer,
		ContextLogger: *logging.NewContextLogger(context.Background(), mockContainer.Logger),
	}

	err := productHandler(ctx)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestOrderSubscribe_BindError(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)

	ctx := &gofr.Context{
		Request:       &errorRequest{},
		Container:     mockContainer,
		ContextLogger: *logging.NewContextLogger(context.Background(), mockContainer.Logger),
	}

	err := orderHandler(ctx)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

type successProductRequest struct{}

func (r *successProductRequest) Context() context.Context {
	return context.Background()
}

func (r *successProductRequest) Bind(v interface{}) error {
	p := v.(*struct {
		ProductId string `json:"productId"`
		Price     string `json:"price"`
	})
	p.ProductId = "123"
	p.Price = "599"
	return nil
}

func (r *successProductRequest) Param(string) string     { return "" }
func (r *successProductRequest) PathParam(string) string { return "" }
func (r *successProductRequest) HostName() string        { return "" }
func (r *successProductRequest) Params(string) []string  { return nil }

func TestProductHandler_Success(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)
	ctx := &gofr.Context{
		Request:       &successProductRequest{},
		Container:     mockContainer,
		ContextLogger: *logging.NewContextLogger(context.Background(), mockContainer.Logger),
	}

	err := productHandler(ctx)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

type successOrderRequest struct{}

func (r *successOrderRequest) Context() context.Context {
	return context.Background()
}

func (r *successOrderRequest) Bind(v interface{}) error {
	o := v.(*struct {
		OrderId string `json:"orderId"`
		Status  string `json:"status"`
	})
	o.OrderId = "456"
	o.Status = "pending"
	return nil
}
func (r *successOrderRequest) Param(string) string     { return "" }
func (r *successOrderRequest) PathParam(string) string { return "" }
func (r *successOrderRequest) HostName() string        { return "" }
func (r *successOrderRequest) Params(string) []string  { return nil }

func TestOrderHandler_Success(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)
	ctx := &gofr.Context{
		Request:       &successOrderRequest{},
		Container:     mockContainer,
		ContextLogger: *logging.NewContextLogger(context.Background(), mockContainer.Logger),
	}
	err := orderHandler(ctx)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}
