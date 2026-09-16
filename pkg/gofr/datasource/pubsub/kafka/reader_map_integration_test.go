//go:build integration

// Package-level integration stress for the reader map, run against a real
// broker rather than mocks:
//
//	KAFKA_BROKER=localhost:20086 go test -tags integration -race \
//	  -run TestIntegration ./pkg/gofr/datasource/pubsub/kafka/
//
// Build-tagged so the normal suite and CI stay hermetic.
package kafka

import (
	"context"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func brokerFromEnv(t *testing.T) string {
	t.Helper()

	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		t.Skip("KAFKA_BROKER not set")
	}

	return broker
}

func newLiveClient(t *testing.T, group string) *kafkaClient {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	cfg := &Config{
		Brokers:         []string{brokerFromEnv(t)},
		ConsumerGroupID: group,
		BatchSize:       1,
		BatchBytes:      1048576,
		BatchTimeout:    1,
	}

	k := New(cfg, logging.NewMockLogger(logging.ERROR), metrics)
	require.NotNil(t, k, "New returned nil - check the config and the broker address")

	require.Eventually(t, k.isConnected, 30*time.Second, 200*time.Millisecond,
		"broker at %s never became reachable", cfg.Brokers[0])

	return k
}

// publishEventually retries the publish until the broker admits the topic
// exists. CreateTopic returns as soon as the controller accepts the request,
// but the topic's metadata takes a moment to propagate, and a publish issued
// in that window fails with "Unknown Topic Or Partition" - a property of the
// broker, not of the code under test.
func publishEventually(ctx context.Context, t *testing.T, k *kafkaClient, topic string) {
	t.Helper()

	var lastErr error

	require.Eventually(t, func() bool {
		lastErr = k.Publish(ctx, topic, []byte("v"))

		return lastErr == nil
	}, 30*time.Second, 100*time.Millisecond, "publish to %s never succeeded: %v", topic, lastErr)
}

// TestIntegration_PublishSubscribe_RoundTrip is the end-to-end sanity check:
// a message published through the GoFr client comes back out of Subscribe on
// the same topic, with the committer wired to the right reader.
func TestIntegration_PublishSubscribe_RoundTrip(t *testing.T) {
	topic := "gofr-e2e-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	k := newLiveClient(t, "gofr-e2e-group")
	t.Cleanup(func() { _ = k.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	require.NoError(t, k.CreateTopic(ctx, topic))

	// Retry rather than publishing once: a broker that is still settling a
	// previous test's topics can take a moment to admit this one, and the
	// bare publish then fails with "Unknown Topic Or Partition".
	require.Eventually(t, func() bool {
		return k.Publish(ctx, topic, []byte("hello-gofr")) == nil
	}, 30*time.Second, 100*time.Millisecond, "publish to %s never succeeded", topic)

	msg, err := k.Subscribe(ctx, topic)
	require.NoError(t, err)
	require.Equal(t, topic, msg.Topic)
	require.Equal(t, "hello-gofr", string(msg.Value))
	require.NotNil(t, msg.Committer, "the committer must be wired to this topic's reader")

	msg.Commit()
}

// TestIntegration_ConcurrentSubscribeAndClose is the stress case that the
// #3500 panic was reported from: many goroutines subscribing to distinct
// topics (each one growing the reader map) while teardown walks that map.
//
// Before the fix, either the unguarded range in Close or the unguarded
// re-read at the end of Subscribe could observe a map mid-write, which the
// runtime reports as "concurrent map read and map write" and -race reports
// as a data race on the map header.
func TestIntegration_ConcurrentSubscribeAndClose(t *testing.T) {
	const (
		warmTopics  = 3
		coldWriters = 8
		coldEach    = 8
	)

	prefix := "gofr-stress-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-"

	k := newLiveClient(t, "gofr-stress-group")
	t.Cleanup(func() { _ = k.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	// Only a few topics get a message. Those prove delivery still works while
	// the map is churning; the cold topics below exist purely to force the
	// map writes, and a Subscribe that times out on an empty partition has
	// already done the write by then.
	//
	// Each warm topic is subscribed once here, before any churn, so its
	// reader has already joined the consumer group. Without that warm-up the
	// group is still rebalancing when the cold topics pile in, and a first
	// join can take longer than the read deadline - which measures Kafka's
	// coordinator, not this package's locking.
	for i := range warmTopics {
		topic := prefix + "warm-" + strconv.Itoa(i)
		require.NoError(t, k.CreateTopic(ctx, topic))
		publishEventually(ctx, t, k, topic)

		joinCtx, joinCancel := context.WithTimeout(ctx, 60*time.Second)

		msg, err := k.Subscribe(joinCtx, topic)
		require.NoError(t, err, "warm-up subscribe on %s", topic)
		require.Equal(t, topic, msg.Topic)

		joinCancel()

		// A second message for the concurrent phase below to consume.
		publishEventually(ctx, t, k, topic)
	}

	var (
		wg        sync.WaitGroup
		delivered atomic.Int64
		closes    atomic.Int64
	)

	for i := range warmTopics {
		wg.Add(1)

		go func() {
			defer wg.Done()

			topic := prefix + "warm-" + strconv.Itoa(i)

			readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
			defer readCancel()

			if msg, err := k.Subscribe(readCtx, topic); err == nil && msg.Topic == topic {
				delivered.Add(1)
			}
		}()
	}

	// Each cold writer walks its own set of fresh topics, so every iteration
	// takes k.mu for write and grows the reader map underneath the warm
	// subscribers above and the teardown goroutine below.
	for w := range coldWriters {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range coldEach {
				topic := prefix + "cold-" + strconv.Itoa(w) + "-" + strconv.Itoa(i)

				readCtx, readCancel := context.WithTimeout(ctx, 250*time.Millisecond)
				_, _ = k.Subscribe(readCtx, topic)

				readCancel()
			}
		}()
	}

	// Concurrent teardown of a client that is still growing its reader map.
	// Health is read alongside it because it takes connMu, so this also
	// covers the two mutexes being taken from different goroutines at once.
	wg.Add(1)

	go func() {
		defer wg.Done()

		for range coldWriters * coldEach {
			_ = k.Health()

			closes.Add(1)

			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	require.Equal(t, int64(warmTopics), delivered.Load(),
		"every warm subscriber should have received its message despite the map churn")
	t.Logf("delivered %d/%d warm messages while %d cold topics grew the reader map, %d health probes",
		delivered.Load(), warmTopics, coldWriters*coldEach, closes.Load())
}
