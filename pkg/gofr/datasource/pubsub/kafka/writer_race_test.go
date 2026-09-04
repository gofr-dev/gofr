package kafka

import (
	"context"
	"sync"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

// TestWriter_ConcurrentReconnectAndPublish_NoRace pins k.writer.
//
// initialize publishes the writer, and retryConnect calls initialize from its
// own goroutine, so that pointer swap runs concurrently with Publish, Close
// and Health reading it. It used to be written and read with no lock at all -
// a single-pointer race rather than the reader-map panic, but a race all the
// same. Every access now goes through connMu, the lock that already guards
// the conn published beside it.
func TestWriter_ConcurrentReconnectAndPublish_NoRace(t *testing.T) {
	const iterations = 200

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	k := &kafkaClient{
		config:  Config{Brokers: []string{"broker.invalid:0"}, BatchSize: 1, BatchBytes: 1, BatchTimeout: 1},
		logger:  logging.NewMockLogger(logging.ERROR),
		metrics: metrics,
		reader:  make(map[string]Reader),
		writer:  &mockWriter{},
		conn:    &multiConn{conns: []Connection{&MockConn{addr: "127.0.0.1:9092", isHealthy: true}}},
	}

	original := connectToBrokers

	t.Cleanup(func() { connectToBrokers = original })

	connectToBrokers = func(context.Context, []string, *kafka.Dialer, pubsub.Logger) ([]Connection, error) {
		return []Connection{&MockConn{addr: "127.0.0.1:9092", isHealthy: true}}, nil
	}

	ctx := t.Context()

	var wg sync.WaitGroup

	wg.Add(3)

	go func() { // the reconnect that swaps k.writer
		defer wg.Done()

		for range iterations {
			_ = k.initialize(ctx)
		}
	}()

	go func() { // Publish reads it
		defer wg.Done()

		for range iterations {
			_ = k.Publish(ctx, "topic", []byte("v"))
		}
	}()

	go func() { // Health reads it too
		defer wg.Done()

		for range iterations {
			_ = k.Health()
		}
	}()

	wg.Wait()
}
