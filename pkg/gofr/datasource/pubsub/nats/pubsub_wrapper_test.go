package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type wrapperMocks struct {
	connManager   *MockConnectionManagerInterface
	subManager    *MockSubscriptionManagerInterface
	streamManager *MockStreamManagerInterface
	jetStream     *MockJetStream
}

func newWrapperWithMocks(t *testing.T) (*PubSubWrapper, *wrapperMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)

	m := &wrapperMocks{
		connManager:   NewMockConnectionManagerInterface(ctrl),
		subManager:    NewMockSubscriptionManagerInterface(ctrl),
		streamManager: NewMockStreamManagerInterface(ctrl),
		jetStream:     NewMockJetStream(ctrl),
	}

	w := &PubSubWrapper{Client: &Client{
		connManager:   m.connManager,
		subManager:    m.subManager,
		streamManager: m.streamManager,
		Config:        &Config{Stream: StreamConfig{Stream: "test-stream"}},
		logger:        logging.NewMockLogger(logging.DEBUG),
	}}

	return w, m
}

type wrapperDelegationCase struct {
	name       string
	setupMocks func(m *wrapperMocks)
	call       func(w *PubSubWrapper) (any, error)
	expResult  any
	expErr     error
}

func TestPubSubWrapper_Delegation(t *testing.T) {
	for _, tt := range defineWrapperDelegationCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			w, m := newWrapperWithMocks(t)
			tt.setupMocks(m)

			result, err := tt.call(w)

			require.ErrorIs(t, err, tt.expErr)
			assert.Equal(t, tt.expResult, result)
		})
	}
}

func defineWrapperDelegationCases(t *testing.T) []wrapperDelegationCase {
	t.Helper()

	msg := pubsub.NewMessage(t.Context())

	return []wrapperDelegationCase{
		{
			name: "Publish forwards to the connection manager",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().IsConnected().Return(true)
				m.connManager.EXPECT().Publish(gomock.Any(), "subject", []byte("payload"), gomock.Any()).Return(errPublishError)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return nil, w.Publish(t.Context(), "subject", []byte("payload"))
			},
			expResult: nil,
			expErr:    errPublishError,
		},
		{
			name: "Subscribe forwards to the subscription manager",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().IsConnected().Return(true)
				m.connManager.EXPECT().JetStream().Return(m.jetStream, nil)
				m.subManager.EXPECT().Subscribe(gomock.Any(), "topic", m.jetStream, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(msg, nil)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return w.Subscribe(t.Context(), "topic")
			},
			expResult: msg,
			expErr:    nil,
		},
		{
			name: "Query forwards to the client",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().IsConnected().Return(true)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return w.Query(t.Context(), "")
			},
			expResult: []byte(nil),
			expErr:    errEmptySubject,
		},
		{
			name: "CreateTopic forwards to the stream manager",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().IsConnected().Return(true)
				m.streamManager.EXPECT().CreateStream(gomock.Any(), &StreamConfig{Stream: "topic", Subjects: []string{"topic"}}).
					Return(errCreateStream)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return nil, w.CreateTopic(t.Context(), "topic")
			},
			expResult: nil,
			expErr:    errCreateStream,
		},
		{
			name: "DeleteTopic forwards to the stream manager",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().IsConnected().Return(true)
				m.streamManager.EXPECT().DeleteStream(gomock.Any(), "topic").Return(errDeleteStream)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return nil, w.DeleteTopic(t.Context(), "topic")
			},
			expResult: nil,
			expErr:    errDeleteStream,
		},
		{
			name: "Close closes subscriptions and the connection",
			setupMocks: func(m *wrapperMocks) {
				m.subManager.EXPECT().Close()
				m.connManager.EXPECT().Close(gomock.Any())
			},
			call: func(w *PubSubWrapper) (any, error) {
				return nil, w.Close()
			},
			expResult: nil,
			expErr:    nil,
		},
		{
			name: "Health forwards to the client",
			setupMocks: func(m *wrapperMocks) {
				m.connManager.EXPECT().Health().Return(datasource.Health{Status: datasource.StatusDown, Details: map[string]any{}})
				m.connManager.EXPECT().JetStream().Return(nil, errJetStreamNotConfigured)
			},
			call: func(w *PubSubWrapper) (any, error) {
				return w.Health(), nil
			},
			expResult: datasource.Health{
				Status: datasource.StatusDown,
				Details: map[string]any{
					"backend":           natsBackend,
					"jetstream_enabled": false,
					"jetstream_status":  jetStreamStatusError + ": " + errJetStreamNotConfigured.Error(),
				},
			},
			expErr: nil,
		},
	}
}

func TestPubSubWrapper_UseDependencies(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)
	metrics := NewMockMetrics(gomock.NewController(t))
	tracer := noop.NewTracerProvider().Tracer("test")

	w := &PubSubWrapper{Client: &Client{}}

	w.UseLogger(logger)
	w.UseMetrics(metrics)
	w.UseTracer(tracer)

	assert.Equal(t, logger, w.Client.logger)
	assert.Equal(t, metrics, w.Client.metrics)
	assert.Equal(t, tracer, w.Client.tracer)
}

func TestPubSubWrapper_Connect(t *testing.T) {
	validConfig := &Config{
		Server:   NATSServer,
		Stream:   StreamConfig{Stream: "test-stream", Subjects: []string{"test-subject"}},
		Consumer: "test-consumer",
	}

	tests := []struct {
		name           string
		config         *Config
		setupMocks     func(ctrl *gomock.Controller, client *Client)
		expStdout      string
		expStderr      string
		expReconnected bool
	}{
		{
			name:   "already connected skips reconnecting",
			config: validConfig,
			setupMocks: func(ctrl *gomock.Controller, client *Client) {
				connManager := NewMockConnectionManagerInterface(ctrl)
				connManager.EXPECT().Health().Return(datasource.Health{Status: datasource.StatusUp})
				client.connManager = connManager
			},
			expStdout:      "NATS connection already established",
			expStderr:      "",
			expReconnected: false,
		},
		{
			name:           "invalid configuration is logged",
			config:         &Config{},
			setupMocks:     func(*gomock.Controller, *Client) {},
			expStdout:      "connecting to NATS server at",
			expStderr:      "PubSubWrapper: Error connecting to NATS: " + errServerNotProvided.Error(),
			expReconnected: false,
		},
		{
			name:   "down connection is re-established",
			config: validConfig,
			setupMocks: func(ctrl *gomock.Controller, client *Client) {
				connManager := NewMockConnectionManagerInterface(ctrl)
				connManager.EXPECT().Health().Return(datasource.Health{Status: datasource.StatusDown})
				client.connManager = connManager

				connector := NewMockNATSConnector(ctrl)
				jsCreator := NewMockJetStreamCreator(ctrl)
				conn := NewMockConnInterface(ctrl)

				connector.EXPECT().Connect(NATSServer, gomock.Any()).Return(conn, nil)
				jsCreator.EXPECT().New(conn).Return(NewMockJetStream(ctrl), nil)

				client.natsConnector = connector
				client.jetStreamCreator = jsCreator
			},
			expStdout:      "connected to NATS server '" + NATSServer + "'",
			expStderr:      "",
			expReconnected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var w *PubSubWrapper

			var stdout string

			stderr := testutil.StderrOutputForFunc(func() {
				stdout = testutil.StdoutOutputForFunc(func() {
					w = New(tt.config, logging.NewMockLogger(logging.DEBUG))
					tt.setupMocks(ctrl, w.Client)

					w.Connect()
				})
			})

			assert.Contains(t, stdout, tt.expStdout)
			assert.Contains(t, stderr, tt.expStderr)
			assert.Equal(t, tt.expStderr == "", stderr == "", "error output must appear only when expected: %q", stderr)

			_, reconnected := w.Client.connManager.(*ConnectionManager)
			assert.Equal(t, tt.expReconnected, reconnected)
		})
	}
}
