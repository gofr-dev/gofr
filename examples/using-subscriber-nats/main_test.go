package main

import (
	"context"
	"strings"
	"testing"
	"time"

	nc "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub/nats"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type mockMetrics struct{}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {}

// Wrapper struct for *nc.Conn that implements nats.ConnInterface
type connWrapper struct {
	*nc.Conn
}

// Implement the NatsConn method for the wrapper
func (w *connWrapper) NatsConn() *nc.Conn {
	return w.Conn
}

func initializeTest(t *testing.T, serverURL string) {
	mockNatsConnect := func(serverURL string, opts ...nc.Option) (nats.ConnInterface, error) {
		conn, err := nc.Connect(serverURL, opts...)
		if err != nil {
			return nil, err
		}
		return &connWrapper{conn}, nil
	}

	mockJetstreamNew := func(nc *nc.Conn) (jetstream.JetStream, error) {
		return jetstream.New(nc)
	}

	client, err := nats.NewNATSClient(&nats.Config{
		Server: serverURL,
		Stream: nats.StreamConfig{
			Stream:  "sample-stream",
			Subject: "order-logs",
		},
	}, logging.NewMockLogger(logging.INFO), &mockMetrics{}, mockNatsConnect, mockJetstreamNew)
	if err != nil {
		t.Fatalf("Error initializing NATS client: %v", err)
	}

	ctx := context.Background()

	orderLogsConfig := jetstream.StreamConfig{
		Name:     "order-logs",
		Subjects: []string{"order-logs"},
	}
	s, err := client.CreateOrUpdateStream(ctx, orderLogsConfig)
	if err != nil {
		t.Fatalf("Error creating stream 'order-logs': %v", err)
	}
	t.Logf("Created stream: %v", s)

	productsConfig := nats.StreamConfig{
		Stream:  "products",
		Subject: "products",
	}
	err = client.CreateStream(context.Background(), productsConfig)
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
	// Start the embedded NATS server
	embeddedServer, err := nats.RunEmbeddedNATSServer()
	if err != nil {
		t.Fatalf("Failed to start embedded NATS server: %v", err)
	}
	defer embeddedServer.Shutdown()

	serverURL := embeddedServer.ClientURL()

	log := testutil.StdoutOutputForFunc(func() {
		go main()
		time.Sleep(time.Second * 1) // Giving some time to start the server

		initializeTest(t, serverURL)
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
