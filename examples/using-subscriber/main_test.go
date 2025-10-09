package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

type mockMetrics struct {
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
}

func initializeTest(t *testing.T) {
	c := kafka.New(&kafka.Config{
		Brokers:      []string{"localhost:9092"},
		OffSet:       1,
		BatchSize:    kafka.DefaultBatchSize,
		BatchBytes:   kafka.DefaultBatchBytes,
		BatchTimeout: kafka.DefaultBatchTimeout,
		Partition:    1,
	}, logging.NewMockLogger(logging.INFO), &mockMetrics{})

	err := c.Publish(context.Background(), "order-logs", []byte(`{"data":{"orderId":"123","status":"pending"}}`))
	if err != nil {
		t.Errorf("Error while publishing: %v", err)
	}

	err = c.Publish(context.Background(), "products", []byte(`{"data":{"productId":"123","price":"599"}}`))
	if err != nil {
		t.Errorf("Error while publishing: %v", err)
	}
}

func TestExampleSubscriber(t *testing.T) {
	log := testutil.StdoutOutputForFunc(func() {
		testutil.NewServerConfigs(t)

		go main()
		time.Sleep(time.Second * 30) // Giving some time to start the server

		initializeTest(t)
		time.Sleep(time.Second * 5) // Giving some time to publish events
	})

	testCases := []struct {
		desc        string
		expectedLog string
	}{
		{
			desc:        "valid order",
			expectedLog: "Received order",
		},
		{
			desc:        "valid product",
			expectedLog: "Received product",
		},
	}

	for i, tc := range testCases {
		if !strings.Contains(log, tc.expectedLog) {
			t.Errorf("TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
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
