package nats

import (
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

const embeddedConnTimeout = 10 * time.Second

// RunEmbeddedNATSServer starts a NATS server in embedded mode.
func RunEmbeddedNATSServer() (*server.Server, error) {
	opts := &server.Options{
		Port:      -1, // Random available port
		JetStream: true,
	}

	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go s.Start()

	if !s.ReadyForConnections(embeddedConnTimeout) {
		return nil, ErrEmbeddedNATSServerNotReady
	}

	return s, nil
}
