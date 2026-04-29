package kafka

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

// newStaleClient builds a kafkaClient whose admin probe always fails (stale
// conn) and whose configured broker is RFC 6761 ".invalid", so the reconnect
// path runs deterministically and fast against an unresolvable host.
func newStaleClient(t *testing.T) (*kafkaClient, *MockConnection) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	stale := NewMockConnection(ctrl)
	stale.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()

	k := &kafkaClient{
		dialer: &kafka.Dialer{Timeout: time.Millisecond},
		config: Config{Brokers: []string{"broker.invalid:0"}},
		conn: &multiConn{
			conns: []Connection{stale},
		},
		logger: logging.NewMockLogger(logging.DEBUG),
		mu:     &sync.RWMutex{},
	}

	return k, stale
}

// TestEnsureConnected_ConcurrentCallers_NoRace fans out many goroutines into
// ensureConnected at once. The reconnect always fails (unresolvable host),
// but the test pins down two properties: (a) -race finds no data race on
// k.conn / k.dialer despite concurrent reads via isConnected and writes via
// reconnectAdminLocked, and (b) every caller sees a consistent boolean
// result rather than a torn state.
func TestEnsureConnected_ConcurrentCallers_NoRace(t *testing.T) {
	k, _ := newStaleClient(t)

	const goroutines = 64

	var (
		wg     sync.WaitGroup
		falses int64
	)

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			if !k.ensureConnected(t.Context()) {
				atomic.AddInt64(&falses, 1)
			}
		}()
	}

	wg.Wait()

	// Reconnect can never succeed against an unresolvable broker, so every
	// caller must observe false — never true (which would indicate the
	// double-checked locking misclassified state).
	assert.Equal(t, int64(goroutines), atomic.LoadInt64(&falses))
}

// TestEnsureConnected_AdminMethods_NoRace exercises the admin entry points
// (Controller, CreateTopic, DeleteTopic) concurrently with ensureConnected.
// reconnectAdminLocked swaps and closes k.conn under connMu.Lock; the admin
// methods take connMu.RLock. -race is the actual assertion: a missing lock
// on either side would be flagged.
func TestEnsureConnected_AdminMethods_NoRace(t *testing.T) {
	k, _ := newStaleClient(t)

	const iters = 200

	var wg sync.WaitGroup

	wg.Add(4)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.ensureConnected(t.Context())
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_, _ = k.Controller()
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.CreateTopic(t.Context(), "x")
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.DeleteTopic(t.Context(), "x")
		}
	}()

	wg.Wait()
}

// TestEnsureConnected_HealthConcurrent_NoRace pins that Health() — which
// reads k.conn and the inner k.conn.mu lock — does not race with the swap
// performed by reconnectAdminLocked. Without connMu around the Health body,
// the reconnect path could free the multiConn while Health was still reading
// it.
func TestEnsureConnected_HealthConcurrent_NoRace(t *testing.T) {
	k, stale := newStaleClient(t)

	// Health walks each broker conn and calls ReadPartitions / RemoteAddr
	// / Controller. The mock from newStaleClient only stubs Controller, so
	// flesh out the rest with permissive expectations.
	stale.EXPECT().ReadPartitions(gomock.Any()).Return(nil, errClientNotConnected).AnyTimes()
	stale.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9092}).AnyTimes()

	// Health uses k.writer.Stats(); supply a no-op writer so the call
	// path completes without nil deref.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	w := NewMockWriter(ctrl)
	w.EXPECT().Stats().Return(kafka.WriterStats{}).AnyTimes()
	k.writer = w

	const iters = 100

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.ensureConnected(t.Context())
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.Health()
		}
	}()

	wg.Wait()
}

// TestGetNewReader_ConcurrentReconnect_NoRace exercises getNewReader (which
// reads k.dialer) concurrently with reconnectAdminLocked (which writes it).
// Subscribe routes through getNewReader while holding the reader-map lock,
// so this is the realistic interleaving we need -race to clear. Calling
// getNewReader directly also avoids Subscribe's 10 s sleep-on-retry, which
// would make a goroutine-storm test impractically slow.
func TestGetNewReader_ConcurrentReconnect_NoRace(t *testing.T) {
	k, _ := newStaleClient(t)
	k.config.ConsumerGroupID = "g1"

	const iters = 200

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			r := k.getNewReader("t")
			_ = r.Close()
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = k.ensureConnected(t.Context())
		}
	}()

	wg.Wait()
}

// TestClose_ConcurrentEnsureConnected_NoRace runs Close while ensureConnected
// is in flight. Close takes connMu.Lock and nils k.conn; ensureConnected
// must see either the live or nil pointer atomically — never a torn read —
// and must not double-close.
func TestClose_ConcurrentEnsureConnected_NoRace(t *testing.T) {
	k, _ := newStaleClient(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	w := NewMockWriter(ctrl)
	w.EXPECT().Close().Return(nil).AnyTimes()
	w.EXPECT().Stats().Return(kafka.WriterStats{}).AnyTimes()
	k.writer = w

	// The mock Connection in newStaleClient must accept a Close call from
	// the multiConn shutdown path triggered by k.Close().
	for _, c := range k.conn.conns {
		mc, ok := c.(*MockConnection)
		if !ok {
			continue
		}

		mc.EXPECT().Close().Return(nil).AnyTimes()
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := 0; i < 50; i++ {
			_ = k.ensureConnected(t.Context())
		}
	}()

	go func() {
		defer wg.Done()

		// Stagger Close slightly so it lands during the ensureConnected
		// loop above.
		time.Sleep(time.Millisecond)

		_ = k.Close()
	}()

	wg.Wait()

	// After Close, k.conn must be nil — confirms the lock-protected swap
	// landed and was not undone by a concurrent reconnectAdmin.
	k.connMu.RLock()
	defer k.connMu.RUnlock()

	assert.Nil(t, k.conn)
}
