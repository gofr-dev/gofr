package kafka

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

// newReaderMapClient builds a client whose admin probe succeeds, so Subscribe
// gets past ensureConnected and reaches the reader map, and whose broker is
// RFC 6761 ".invalid" so any reader built by getNewReader fails to dial fast
// instead of blocking the test.
//
// warmTopics are pre-populated with mock readers that return a message
// immediately: a Subscribe on one of those takes the "reader already exists"
// path and runs all the way to the end of the function without touching the
// network. A Subscribe on any other topic takes the "create it" path and
// writes the map. Racing the two is the whole point.
func newReaderMapClient(t *testing.T, warmTopics ...string) *kafkaClient {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	live := NewMockConnection(ctrl)
	live.EXPECT().Controller().Return(kafka.Broker{}, nil).AnyTimes()
	live.EXPECT().Close().Return(nil).AnyTimes()

	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	k := &kafkaClient{
		dialer:  &kafka.Dialer{Timeout: time.Millisecond},
		config:  Config{Brokers: []string{"broker.invalid:0"}, ConsumerGroupID: "reader-map-race"},
		conn:    &multiConn{conns: []Connection{live}},
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: metrics,
		reader:  make(map[string]Reader),
	}

	for _, topic := range warmTopics {
		r := NewMockReader(ctrl)
		r.EXPECT().FetchMessage(gomock.Any()).
			Return(kafka.Message{Topic: topic, Value: []byte("v")}, nil).AnyTimes()
		r.EXPECT().Close().Return(nil).AnyTimes()

		k.reader[topic] = r
	}

	// Subscribe on a cold topic builds a real kafka.Reader, which runs its own
	// background goroutines until closed. Close them all rather than leaking
	// one per topic into the rest of the package's tests.
	t.Cleanup(func() { _ = k.Close() })

	return k
}

// TestSubscribe_ConcurrentNewTopic_NoRace pins the unguarded map read that
// used to sit at the end of Subscribe.
//
// Subscribe captures the reader into a local while holding k.mu, releases the
// lock, and then built the committer from k.reader[topic] a second time. That
// second read is outside the lock, so a concurrent Subscribe on a *different*
// topic taking the write lock to grow the map races it.
//
// The warm goroutine supplies the read (its topic already has a reader, so it
// runs to the committer line); the cold goroutines supply the write (each of
// their topics is new, so each one assigns into the map). Without the fix this
// fails under -race on the k.reader map.
func TestSubscribe_ConcurrentNewTopic_NoRace(t *testing.T) {
	const (
		warmTopic  = "warm-topic"
		coldWriter = 4
		iterations = 60
	)

	k := newReaderMapClient(t, warmTopic)
	ctx := t.Context()

	var (
		wg       sync.WaitGroup
		warmErr  error
		warmSeen atomic.Int64
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		// Assert after Wait, not here: require calls FailNow, which is only
		// legal on the goroutine running the test.
		for range iterations {
			msg, err := k.Subscribe(ctx, warmTopic)
			if err != nil {
				warmErr = err

				return
			}

			if msg.Topic == warmTopic {
				warmSeen.Add(1)
			}
		}
	}()

	// The cold path builds a real kafka.Reader against an unresolvable broker,
	// and kafka-go's FetchMessage retries until its context is done. Hand it
	// one that is already canceled: ensureConnected short-circuits on the
	// live admin conn without consulting ctx, so Subscribe still reaches the
	// map write, and FetchMessage then returns immediately instead of hanging.
	coldCtx, cancel := context.WithCancel(ctx)
	cancel()

	for w := range coldWriter {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range iterations {
				// A fresh topic every time, so every call takes the write
				// lock and grows the map.
				_, _ = k.Subscribe(coldCtx, "cold-"+strconv.Itoa(w)+"-"+strconv.Itoa(i))
			}
		}()
	}

	wg.Wait()

	require.NoError(t, warmErr, "the warm subscriber should never fail")
	require.Equal(t, int64(iterations), warmSeen.Load(),
		"every warm Subscribe should have returned a message for its own topic")
}

// TestClose_ConcurrentSubscribe_NoRace pins the other half: Close ranged over
// k.reader with no lock at all, so a Subscribe growing the map during teardown
// produced the "concurrent map read and map write" panic from #3500.
//
// Close is called repeatedly rather than once because the panic needs the
// range to be in flight while the map is written, and a single teardown is a
// narrow window.
func TestClose_ConcurrentSubscribe_NoRace(t *testing.T) {
	const iterations = 300

	// Seeded with readers so the very first Close has a map worth ranging
	// over. Starting from an empty map, the early iterations range over
	// nothing and the detector needs the writer to get ahead first, which
	// made this catch its own mutant only intermittently at -count=1.
	k := newReaderMapClient(t, "seed-0", "seed-1", "seed-2", "seed-3")
	// Close() takes connMu and closes conn; leave it unset so this test stays
	// about the reader map. It also means Subscribe cannot be the writer here
	// - ensureConnected would fail and sleep defaultRetryTimeout (10s) on
	// every call - so the writer below performs the same locked assignment
	// Subscribe does. Close, the code under test, is the real thing.
	k.conn = nil

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := range iterations {
			// ensureConnected fails with conn nil, so drive the map write
			// directly the way Subscribe does under k.mu.
			k.mu.Lock()
			k.reader["grow-"+strconv.Itoa(i)] = k.getNewReader("grow-" + strconv.Itoa(i))
			k.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()

		for range iterations {
			_ = k.Close()
		}
	}()

	wg.Wait()
}

// TestHealth_ConcurrentSubscribe_NoRace pins the third reader of the same map.
//
// getReaderStatsAsMap ranged over k.reader with no lock held, and Health sits
// on the probe path - every /.well-known/health request reaches it - so it
// races a Subscribe growing the map far more often than teardown ever could.
// Found by running the integration stress case against a real broker under
// -race, not by reading the code.
func TestHealth_ConcurrentSubscribe_NoRace(t *testing.T) {
	const iterations = 60

	k := &kafkaClient{
		dialer: &kafka.Dialer{Timeout: time.Millisecond},
		config: Config{Brokers: []string{"broker.invalid:0"}, ConsumerGroupID: "health-race"},
		conn: &multiConn{
			conns: []Connection{&MockConn{addr: "127.0.0.1:9092", isHealthy: true, isControl: true}},
		},
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	t.Cleanup(func() { _ = k.Close() })

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := range iterations {
			topic := "grow-" + strconv.Itoa(i)

			// The same locked assignment Subscribe performs.
			k.mu.Lock()
			k.reader[topic] = k.getNewReader(topic)
			k.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()

		for range iterations {
			_ = k.Health()
		}
	}()

	wg.Wait()
}
