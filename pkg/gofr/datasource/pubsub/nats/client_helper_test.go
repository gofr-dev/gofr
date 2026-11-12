package nats

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// newMsgChan returns a closed channel carrying the given messages, the shape MessageBatch.Messages
// hands back once a fetch has completed.
func newMsgChan(msgs ...jetstream.Msg) chan jetstream.Msg {
	ch := make(chan jetstream.Msg, len(msgs))

	for _, m := range msgs {
		ch <- m
	}

	close(ch)

	return ch
}

func TestClient_handleMessage(t *testing.T) {
	tests := []struct {
		name       string
		handlerErr error
		setupMsg   func(msg *MockMsg)
		expErr     error
		expStderr  string
	}{
		{
			name:     "handled and acknowledged",
			setupMsg: func(msg *MockMsg) { msg.EXPECT().Ack().Return(nil) },
			expErr:   nil,
		},
		{
			name:      "ack failure is returned",
			setupMsg:  func(msg *MockMsg) { msg.EXPECT().Ack().Return(errConnectionError) },
			expErr:    errConnectionError,
			expStderr: "Error sending ACK for message",
		},
		{
			name:       "handler failure is negatively acknowledged",
			handlerErr: errHandlerError,
			setupMsg:   func(msg *MockMsg) { msg.EXPECT().Nak().Return(nil) },
			expErr:     errHandlerError,
			expStderr:  "Error handling message: " + errHandlerError.Error(),
		},
		{
			name:       "nak failure takes precedence over the handler error",
			handlerErr: errHandlerError,
			setupMsg:   func(msg *MockMsg) { msg.EXPECT().Nak().Return(errConnectionError) },
			expErr:     errConnectionError,
			expStderr:  "Error handling message: " + errHandlerError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMockMsg(gomock.NewController(t))
			tt.setupMsg(msg)

			handler := func(context.Context, jetstream.Msg) error { return tt.handlerErr }

			var err error

			stderr := testutil.StderrOutputForFunc(func() {
				client := &Client{logger: logging.NewMockLogger(logging.DEBUG)}
				err = client.handleMessage(t.Context(), msg, handler)
			})

			require.ErrorIs(t, err, tt.expErr)
			assert.Contains(t, stderr, tt.expStderr)
			assert.Equal(t, tt.expStderr == "", stderr == "")
		})
	}
}

func TestClient_fetchAndProcessMessages(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(ctrl *gomock.Controller, cons *MockConsumer)
		expErr     error
		expStderr  string
	}{
		{
			name: "deadline exceeded is returned without logging",
			setupMocks: func(_ *gomock.Controller, cons *MockConsumer) {
				cons.EXPECT().Fetch(1, gomock.Any()).Return(nil, context.DeadlineExceeded)
			},
			expErr: context.DeadlineExceeded,
		},
		{
			name: "fetch failure is logged and returned",
			setupMocks: func(_ *gomock.Controller, cons *MockConsumer) {
				cons.EXPECT().Fetch(1, gomock.Any()).Return(nil, errConnectionError)
			},
			expErr:    errConnectionError,
			expStderr: "Error fetching messages for subject test.subject",
		},
		{
			name: "handler failure is logged, batch error is returned",
			setupMocks: func(ctrl *gomock.Controller, cons *MockConsumer) {
				msg := NewMockMsg(ctrl)
				msg.EXPECT().Nak().Return(nil)

				batch := NewMockMessageBatch(ctrl)
				batch.EXPECT().Messages().Return(newMsgChan(msg))
				batch.EXPECT().Error().Return(errSubscriptionError)

				cons.EXPECT().Fetch(1, gomock.Any()).Return(batch, nil)
			},
			expErr:    errSubscriptionError,
			expStderr: "Error in message batch for subject test.subject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cons := NewMockConsumer(ctrl)
			tt.setupMocks(ctrl, cons)

			handler := func(context.Context, jetstream.Msg) error { return errHandlerError }

			var err error

			stderr := testutil.StderrOutputForFunc(func() {
				client := &Client{logger: logging.NewMockLogger(logging.DEBUG), Config: &Config{}}
				err = client.fetchAndProcessMessages(t.Context(), cons, "test.subject", handler)
			})

			require.ErrorIs(t, err, tt.expErr)
			assert.Contains(t, stderr, tt.expStderr)
			assert.Equal(t, tt.expStderr == "", stderr == "")
		})
	}
}

// TestClient_processMessages_LogsLoopError runs the loop synchronously: the fetch cancels the
// context, so the loop logs the failure once and exits.
func TestClient_processMessages_LogsLoopError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cons := NewMockConsumer(gomock.NewController(t))
	cons.EXPECT().Fetch(1, gomock.Any()).DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
		cancel()

		return nil, errConnectionError
	})

	stderr := testutil.StderrOutputForFunc(func() {
		client := &Client{logger: logging.NewMockLogger(logging.DEBUG), Config: &Config{}}
		client.processMessages(ctx, cons, "test.subject", func(context.Context, jetstream.Msg) error { return nil })
	})

	assert.Contains(t, stderr, "Error in message processing loop for subject test.subject: "+errConnectionError.Error())
}

func TestFetchBatch(t *testing.T) {
	tests := []struct {
		name     string
		fetchErr error
		expErr   error
		expBatch bool
	}{
		{name: "deadline exceeded means no messages", fetchErr: context.DeadlineExceeded, expErr: nil, expBatch: false},
		{name: "canceled means no messages", fetchErr: context.Canceled, expErr: nil, expBatch: false},
		{name: "other errors are returned", fetchErr: errConnectionError, expErr: errConnectionError, expBatch: false},
		{name: "batch is returned", fetchErr: nil, expErr: nil, expBatch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cons := NewMockConsumer(ctrl)
			batch := NewMockMessageBatch(ctrl)

			cons.EXPECT().Fetch(5, gomock.Any()).Return(batch, tt.fetchErr)

			msgs, err := fetchBatch(cons, 5, time.Millisecond, logging.NewMockLogger(logging.DEBUG))

			require.ErrorIs(t, err, tt.expErr)
			assert.Equal(t, tt.expBatch, msgs != nil)
		})
	}
}

func TestProcessBatch(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name         string
		ctx          context.Context
		data         []string
		ackErr       error
		initial      []byte
		limit        int
		expResult    []byte
		expCollected int
		expMore      bool
		expStdout    string
	}{
		{
			name:         "empty batch stops collection",
			ctx:          t.Context(),
			limit:        5,
			expResult:    nil,
			expCollected: 0,
			expMore:      false,
		},
		{
			name:         "messages are joined with newlines",
			ctx:          t.Context(),
			data:         []string{"a", "b"},
			initial:      []byte("x"),
			limit:        5,
			expResult:    []byte("x\na\nb"),
			expCollected: 2,
			expMore:      true,
		},
		{
			name:         "stops at the limit",
			ctx:          t.Context(),
			data:         []string{"a", "b", "c"},
			limit:        2,
			expResult:    []byte("a\nb"),
			expCollected: 2,
			expMore:      true,
		},
		{
			name:         "ack failure is logged and collection continues",
			ctx:          t.Context(),
			data:         []string{"a"},
			ackErr:       errConnectionError,
			limit:        5,
			expResult:    []byte("a"),
			expCollected: 1,
			expMore:      true,
			expStdout:    "Error acknowledging message",
		},
		{
			name:         "canceled context stops collection",
			ctx:          canceled,
			data:         []string{"a"},
			limit:        5,
			expResult:    []byte("a"),
			expCollected: 1,
			expMore:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			msgs := make([]jetstream.Msg, 0, len(tt.data))

			for _, d := range tt.data {
				msg := NewMockMsg(ctrl)
				msg.EXPECT().Data().Return([]byte(d)).AnyTimes()
				msg.EXPECT().Ack().Return(tt.ackErr).AnyTimes()
				msgs = append(msgs, msg)
			}

			batch := NewMockMessageBatch(ctrl)
			batch.EXPECT().Messages().Return(newMsgChan(msgs...))

			result := tt.initial
			collected := 0

			var more bool

			stdout := testutil.StdoutOutputForFunc(func() {
				more = processBatch(tt.ctx, batch, &result, &collected, tt.limit, logging.NewMockLogger(logging.DEBUG))
			})

			assert.Equal(t, tt.expResult, result)
			assert.Equal(t, tt.expCollected, collected)
			assert.Equal(t, tt.expMore, more)
			assert.Equal(t, tt.expStdout == "", stdout == "", "log output must appear only when expected: %q", stdout)
			assert.Contains(t, stdout, tt.expStdout)
		})
	}
}

func TestCollectMessages(t *testing.T) {
	tests := []struct {
		name          string
		filterSubject string
		fetchErr      error
		expResult     []byte
		expErr        error
		expStdout     string
	}{
		{
			name:          "fetch failure is returned",
			filterSubject: "orders",
			fetchErr:      errConnectionError,
			expResult:     nil,
			expErr:        errConnectionError,
		},
		{
			name:          "no messages on a regular subject",
			filterSubject: "orders",
			expResult:     nil,
			expErr:        nil,
		},
		{
			name:          "no migration records is reported",
			filterSubject: goFrNatsStreamName,
			expResult:     nil,
			expErr:        nil,
			expStdout:     "No migration records found in stream test-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			batch := NewMockMessageBatch(ctrl)
			batch.EXPECT().Messages().Return(newMsgChan()).AnyTimes()

			cons := NewMockConsumer(ctrl)
			cons.EXPECT().Fetch(3, gomock.Any()).Return(batch, tt.fetchErr)
			cons.EXPECT().CachedInfo().Return(&jetstream.ConsumerInfo{
				Config: jetstream.ConsumerConfig{FilterSubject: tt.filterSubject},
			}).AnyTimes()

			var (
				result []byte
				err    error
			)

			cfg := &Config{Stream: StreamConfig{Stream: "test-stream"}}

			stdout := testutil.StdoutOutputForFunc(func() {
				result, err = collectMessages(t.Context(), cons, 3, cfg, logging.NewMockLogger(logging.DEBUG))
			})

			require.ErrorIs(t, err, tt.expErr)
			assert.Equal(t, tt.expResult, result)
			assert.Equal(t, tt.expStdout == "", stdout == "", "log output must appear only when expected: %q", stdout)
			assert.Contains(t, stdout, tt.expStdout)
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		name string
		a, b int
		exp  int
	}{
		{name: "first smaller", a: 1, b: 2, exp: 1},
		{name: "second smaller", a: 3, b: 2, exp: 2},
		{name: "equal", a: 2, b: 2, exp: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, minInt(tt.a, tt.b))
		})
	}
}

func TestCreateQueryContext_KeepsCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	want, _ := ctx.Deadline()

	queryCtx, queryCancel := createQueryContext(ctx, time.Second)
	defer queryCancel()

	got, ok := queryCtx.Deadline()

	require.True(t, ok)
	assert.Equal(t, want, got, "a caller deadline must not be replaced by the query timeout")
	assert.Equal(t, ctx, queryCtx)
}

func TestClient_getStreamName(t *testing.T) {
	client := &Client{Config: &Config{Stream: StreamConfig{Stream: "test-stream"}}}

	tests := []struct {
		name  string
		query string
		exp   string
	}{
		{name: "migrations use their own stream", query: goFrNatsStreamName, exp: goFrNatsStreamName},
		{name: "other subjects use the configured stream", query: "orders", exp: "test-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, client.getStreamName(tt.query))
		})
	}
}

func TestClient_cleanupConsumer(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(js *MockJetStream, cons *MockConsumer)
		expStdout  string
	}{
		{
			name: "consumer info unavailable skips deletion",
			setupMocks: func(_ *MockJetStream, cons *MockConsumer) {
				cons.EXPECT().Info(gomock.Any()).Return(nil, errConnectionError)
			},
		},
		{
			name: "delete failure is logged",
			setupMocks: func(js *MockJetStream, cons *MockConsumer) {
				cons.EXPECT().Info(gomock.Any()).Return(&jetstream.ConsumerInfo{Name: "tmp"}, nil)
				js.EXPECT().DeleteConsumer(gomock.Any(), "test-stream", "tmp").Return(errConnectionError)
			},
			expStdout: "failed to delete temporary consumer: " + errConnectionError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			js := NewMockJetStream(ctrl)
			cons := NewMockConsumer(ctrl)
			tt.setupMocks(js, cons)

			stdout := testutil.StdoutOutputForFunc(func() {
				client := &Client{logger: logging.NewMockLogger(logging.DEBUG)}
				client.cleanupConsumer(js, "test-stream", cons)
			})

			assert.Equal(t, tt.expStdout == "", stdout == "", "log output must appear only when expected: %q", stdout)
			assert.Contains(t, stdout, tt.expStdout)
		})
	}
}
