package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/datasource/pubsub/nats"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"google.golang.org/appengine/log"
)

type mockMetrics struct {
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
}

func initializeTest(t *testing.T) {
	client, err := nats.NewNATSClient(&nats.Config{
		Server: "nats://localhost:4222",
		Stream: nats.StreamConfig{
			Stream:  "sample-stream",
			Subject: "order-logs",
		},
	}, logging.NewMockLogger(logging.INFO), &mockMetrics{})
	if err != nil {
		t.Fatalf("Error initializing NATS client: %v", err)
	}

	ctx := context.Background()

	s, err := client.CreateOrUpdateStream(ctx, "order-logs")
	if err != nil {
		t.Fatalf("Error creating stream 'order-logs': %v", err)
	}
	log.Debugf(ctx, "Created stream: %v", s)

	err = client.CreateStream(context.Background(), "products")
	if err != nil {
		t.Fatalf("Error creating stream 'products': %v", err)
	}

	// Publish messages
	err = client.Publish(context.Background(), "order-logs", []byte(`{"orderId":"123","status":"pending"}`))
	if err != nil {
		t.Errorf("Error while publishing to 'order-logs': %v", err)
	}

	err = client.Publish(context.Background(), "products", []byte(`{"productId":"123","price":"599"}`))
	if err != nil {
		t.Errorf("Error while publishing to 'products': %v", err)
	}
}

func TestExampleSubscriber(t *testing.T) {
	log := testutil.StdoutOutputForFunc(func() {
		go main()
		time.Sleep(time.Second * 1) // Giving some time to start the server

		initializeTest(t)
		time.Sleep(time.Second * 20) // Giving some time to publish events
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
