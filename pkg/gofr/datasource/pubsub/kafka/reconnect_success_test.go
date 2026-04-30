package kafka

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

// stubConnectToBrokers replaces the package-level connectToBrokers var for
// the duration of the test. The replacement returns the supplied conns and
// nil error; production-side validation (empty broker list, all-fail, ...)
// is bypassed because we want to exercise the post-dial swap path.
func stubConnectToBrokers(t *testing.T, conns []Connection) {
	t.Helper()

	original := connectToBrokers

	t.Cleanup(func() { connectToBrokers = original })

	connectToBrokers = func(_ context.Context, _ []string, _ *kafka.Dialer, _ pubsub.Logger) ([]Connection, error) {
		return conns, nil
	}
}

// TestEnsureConnected_ReconnectSuccess pins the success path that
// TestEnsureConnected_ReconnectsAfterStaleAdminConn does NOT cover: the
// re-dial actually returns a healthy conn, ensureConnected returns true,
// k.conn is swapped to the fresh multiConn, and the stale conn is closed
// exactly once.
func TestEnsureConnected_ReconnectSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Stale conn: probe fails, must be Closed by reconnectAdminLocked.
	stale := NewMockConnection(ctrl)
	stale.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()
	stale.EXPECT().Close().Return(nil).Times(1)

	// Fresh conn returned by the stubbed connectToBrokers — Controller
	// must succeed so the post-swap isConnected probe sees the healed
	// state on the next call.
	fresh := NewMockConnection(ctrl)
	fresh.EXPECT().Controller().Return(kafka.Broker{Host: "broker", Port: 9092}, nil).AnyTimes()

	stubConnectToBrokers(t, []Connection{fresh})

	k := &kafkaClient{
		dialer: &kafka.Dialer{Timeout: time.Millisecond},
		config: Config{Brokers: []string{"broker.invalid:0"}},
		conn: &multiConn{
			conns: []Connection{stale},
		},
		logger: logging.NewMockLogger(logging.DEBUG),
		mu:     &sync.RWMutex{},
		// Pre-arm the throttle so we can assert it gets reset on
		// successful reconnect.
		reconnectErrLogAt: time.Now().Add(time.Hour),
	}

	oldConn := k.conn

	// First call: stale → reconnect succeeds → returns true.
	assert.True(t, k.ensureConnected(t.Context()))

	// k.conn must point at a new multiConn whose conns slice contains
	// fresh, not stale.
	require.NotSame(t, oldConn, k.conn, "k.conn was not swapped")
	require.Len(t, k.conn.conns, 1)
	assert.Same(t, fresh, k.conn.conns[0])

	// Throttle reset on success.
	assert.True(t, k.reconnectErrLogAt.IsZero(), "reconnectErrLogAt should reset on success")

	// Second call: must hit the fast path via isConnected without a
	// second reconnect. (If it tried to reconnect, gomock's Times(1) on
	// stale.Close() would fire twice and fail.)
	assert.True(t, k.ensureConnected(t.Context()))
}

// TestEnsureConnected_ReconnectFailureLogThrottled covers the log-spam
// throttle Copilot asked for: repeated failed reconnect attempts inside the
// throttle window must NOT all be logged at error level.
func TestEnsureConnected_ReconnectFailureLogThrottled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stale := NewMockConnection(ctrl)
	stale.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()

	// Stub connectToBrokers to always fail so each ensureConnected tries
	// (and fails) the reconnect path under the write lock.
	original := connectToBrokers

	t.Cleanup(func() { connectToBrokers = original })

	connectToBrokers = func(context.Context, []string, *kafka.Dialer, pubsub.Logger) ([]Connection, error) {
		return nil, errFailedToConnectBrokers
	}

	k := &kafkaClient{
		dialer: &kafka.Dialer{Timeout: time.Millisecond},
		config: Config{Brokers: []string{"broker.invalid:0"}},
		conn: &multiConn{
			conns: []Connection{stale},
		},
		logger: logging.NewMockLogger(logging.DEBUG),
		mu:     &sync.RWMutex{},
	}

	// First failure: throttle is zero-value, so this must arm it.
	assert.False(t, k.ensureConnected(t.Context()))
	require.False(t, k.reconnectErrLogAt.IsZero(), "first failure should arm the throttle")

	armedAt := k.reconnectErrLogAt

	// Second and third failures within the window: throttle must not
	// move forward (we did not log Errorf, so we did not advance).
	assert.False(t, k.ensureConnected(t.Context()))
	assert.False(t, k.ensureConnected(t.Context()))
	assert.Equal(t, armedAt, k.reconnectErrLogAt, "throttle must not advance on debug-logged failures")
}
