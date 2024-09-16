package main

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr"
	natspubsub "gofr.dev/pkg/gofr/datasource/pubsub/nats"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type mockMetrics struct{}

func (*mockMetrics) IncrementCounter(_ context.Context, _ string, _ ...string) {}

// Wrapper struct for *nats.Conn that implements n.ConnInterface.
type connWrapper struct {
	*nats.Conn
}

// Implement the NatsConn method for the wrapper.
func (w *connWrapper) NatsConn() *nats.Conn {
	return w.Conn
}

func runNATSServer() (*server.Server, error) {
	opts := &server.Options{
		ConfigFile: "configs/nats-server.conf",
		JetStream:  true,
		Port:       -1,
		Trace:      true,
	}

	return server.NewServer(opts)
}

func TestExampleSubscriber(t *testing.T) {
	// Start the embedded NATS server
	natsServer, err := runNATSServer()
	if err != nil {
		t.Fatalf("Failed to start NATS server: %v", err)
	}
	defer natsServer.Shutdown()

	natsServer.Start()

	if !natsServer.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to start")
	}

	serverURL := natsServer.ClientURL()

	// Set environment variable for NATS server URL
	os.Setenv("PUBSUB_BROKER", serverURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logs := testutil.StdoutOutputForFunc(func() {
		// Initialize test data
		initializeTest(t, serverURL)

		// Start the main application
		go runMain(ctx)

		// Wait for messages to be processed
		time.Sleep(10 * time.Second)
	})

	// Cancel the context to stop the application gracefully
	cancel()

	testCases := []struct {
		desc        string
		expectedLog string
	}{
		{
			desc:        "NATS connection",
			expectedLog: "connected to NATS server",
		},
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
		if !strings.Contains(logs, tc.expectedLog) {
			t.Errorf("TEST[%d] Failed.\n%s\nExpected log: %s\nActual logs: %s",
				i, tc.desc, tc.expectedLog, logs)
		}
	}

	// Check for unexpected errors
	if strings.Contains(logs, "subscriber not initialized") {
		t.Errorf("Subscriber initialization error detected in logs")
	}

	if strings.Contains(logs, "failed to connect to NATS server") {
		t.Errorf("NATS connection error detected in logs")
	}
}

func runMain(ctx context.Context) {
	app := gofr.New()

	app.Subscribe("products", func(c *gofr.Context) error {
		var productInfo struct {
			ProductID string `json:"productId"`
			Price     string `json:"price"`
		}

		err := c.Bind(&productInfo)
		if err != nil {
			log.Printf("Error binding product data: %v", err)
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received product", productInfo)

		return nil
	})

	app.Subscribe("order-logs", func(c *gofr.Context) error {
		var orderStatus struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
		}

		err := c.Bind(&orderStatus)
		if err != nil {
			log.Printf("Error binding order data: %v", err)
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received order", orderStatus)

		return nil
	})

	go func() {
		<-ctx.Done()
		log.Println("Context canceled, stopping application")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := app.Shutdown(shutdownCtx)
		if err != nil {
			log.Printf("Error shutting down application: %v", err)
		}
	}()

	log.Println("Starting application")
	app.Run()
	log.Println("Application stopped")
}

func initializeTest(t *testing.T, serverURL string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conf := &natspubsub.Config{
		Server: serverURL,
		Stream: natspubsub.StreamConfig{
			Stream:     "sample-stream",
			Subjects:   []string{"order-logs", "products"},
			MaxDeliver: 4,
		},
		Consumer:    "test-consumer",
		MaxWait:     5 * time.Second,
		MaxPullWait: 5,
	}

	mockMetrics := &mockMetrics{}
	logger := logging.NewMockLogger(logging.DEBUG)

	client, err := natspubsub.NewNATSClient(conf, logger, mockMetrics,
		func(serverURL string, opts ...nats.Option) (natspubsub.ConnInterface, error) {
			conn, err := nats.Connect(serverURL, opts...)
			if err != nil {
				return nil, err
			}

			return &connWrapper{conn}, nil
		},
		func(nc *nats.Conn) (jetstream.JetStream, error) {
			js, err := jetstream.New(nc)
			if err != nil {
				log.Printf("Error creating JetStream: %v", err)

				return nil, err
			}

			return js, nil
		},
	)

	if err != nil {
		t.Fatalf("Error initializing NATS client: %v", err)
	}

	// Ensure stream is created
	_, err = client.Js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     conf.Stream.Stream,
		Subjects: conf.Stream.Subjects,
	})
	if err != nil {
		t.Fatalf("Error creating stream: %v", err)
	}

	err = client.Publish(ctx, "order-logs", []byte(`{"orderId":"123","status":"pending"}`))
	if err != nil {
		t.Errorf("Error publishing to 'order-logs': %v", err)
	}

	err = client.Publish(ctx, "products", []byte(`{"productId":"123","price":"599"}`))
	if err != nil {
		t.Errorf("Error publishing to 'products': %v", err)
	}

	log.Println("Test initialization complete")
}
