package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type mockMetrics struct {
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
}

func initializeTest(t *testing.T) {
	c := kafka.New(kafka.Config{
		Broker:       "localhost:9092",
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
	initializeTest(t)

	log := testutil.StdoutOutputForFunc(func() {
		const host = "http://localhost:8200"
		go main()
		time.Sleep(time.Second * 40)
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
			desc:        "valid  product",
			expectedLog: "Received product",
		},
	}

	for i, tc := range testCases {
		if !strings.Contains(log, tc.expectedLog) {
			t.Errorf("TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}
