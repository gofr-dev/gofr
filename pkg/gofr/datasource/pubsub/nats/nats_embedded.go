package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

func RunEmbeddedNATSServer() (*server.Server, error) {
	opts := &server.Options{
		Port: -1, // Random available port
	}
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		return nil, fmt.Errorf("NATS server did not start in time")
	}
	return s, nil
}
