package nats

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestNATSConnWrapper(t *testing.T) {
	// Start an embedded NATS server
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random available port
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Error starting NATS server: %v", err)
	}

	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS server not ready for connections")
	}

	// Get the server's listen address
	addr := ns.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("nats://%s:%d", addr.IP.String(), addr.Port)

	t.Run("Status", func(t *testing.T) {
		nc, err := nats.Connect(url)
		if err != nil {
			t.Fatal(err)
		}
		defer nc.Close()

		wrapper := &natsConnWrapper{Conn: nc}
		status := wrapper.Status()
		expectedStatus := nats.CONNECTED

		if status != expectedStatus {
			t.Errorf("Expected status %v, got %v", expectedStatus, status)
		}
	})

	t.Run("Close", func(t *testing.T) {
		nc, err := nats.Connect(url)
		if err != nil {
			t.Fatal(err)
		}

		wrapper := &natsConnWrapper{Conn: nc}
		wrapper.Close()

		status := wrapper.Status()
		expectedStatus := nats.CLOSED

		if status != expectedStatus {
			t.Errorf("Expected status %v, got %v", expectedStatus, status)
		}
	})
}
