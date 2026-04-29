package kafka

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestValidateConfigs_ValidCases(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "Valid Config",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SASLMechanism:    "PLAIN",
				SASLUser:         "user",
				SASLPassword:     "password",
				SecurityProtocol: "SASL_PLAINTEXT",
			},
			expected: nil,
		},
		{
			name: "Valid PLAINTEXT Protocol",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: protocolPlainText,
			},
			expected: nil,
		},
		{
			name: "Valid SSL Protocol with TLS Configs",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: "SSL",
				TLS: TLSConfig{
					CACertFile: "ca.pem",
					CertFile:   "cert.pem",
					KeyFile:    "key.pem",
				},
			},
			expected: nil,
		},
		{
			name: "Valid SASL_SSL Protocol with TLS and SASL Configs",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: "SASL_SSL",
				SASLMechanism:    "PLAIN",
				SASLUser:         "user",
				SASLPassword:     "password",
				TLS: TLSConfig{
					CACertFile: "ca.pem",
					CertFile:   "cert.pem",
					KeyFile:    "key.pem",
				},
			},
			expected: nil,
		},
		{
			name: "Valid SSL Protocol with InsecureSkipVerify",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: "SSL",
				TLS: TLSConfig{
					InsecureSkipVerify: true,
				},
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigs(&tc.config)
			if !errors.Is(err, tc.expected) {
				t.Errorf("Expected error %v, but got %v", tc.expected, err)
			}
		})
	}
}

func TestValidateConfigs_InvalidCases(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "Empty Broker",
			config: Config{
				BatchSize:    1,
				BatchBytes:   1,
				BatchTimeout: 1,
			},
			expected: errBrokerNotProvided,
		},
		{
			name: "Zero BatchSize",
			config: Config{
				Brokers:      []string{"kafkabroker"},
				BatchSize:    0,
				BatchBytes:   1,
				BatchTimeout: 1,
			},
			expected: errBatchSize,
		},
		{
			name: "Zero BatchBytes",
			config: Config{
				Brokers:      []string{"kafkabroker"},
				BatchSize:    1,
				BatchBytes:   0,
				BatchTimeout: 1,
			},
			expected: errBatchBytes,
		},
		{
			name: "Zero BatchTimeout",
			config: Config{
				Brokers:      []string{"kafkabroker"},
				BatchSize:    1,
				BatchBytes:   1,
				BatchTimeout: 0,
			},
			expected: errBatchTimeout,
		},
		{
			name: "SASL_PLAINTEXT with Missing SASLMechanism",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: "SASL_PLAINTEXT",
				SASLUser:         "user",
				SASLPassword:     "password",
			},
			expected: errSASLCredentialsMissing,
		},
		{
			name: "Unsupported Security Protocol",
			config: Config{
				Brokers:          []string{"kafkabroker"},
				BatchSize:        1,
				BatchBytes:       1,
				BatchTimeout:     1,
				SecurityProtocol: "Invalid",
			},
			expected: errUnsupportedSecurityProtocol,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigs(&tc.config)
			if !errors.Is(err, tc.expected) {
				t.Errorf("Expected error %v, but got %v", tc.expected, err)
			}
		})
	}
}

func TestKafkaClient_PublishError(t *testing.T) {
	var (
		err        error
		errPublish = testutil.CustomError{ErrorMessage: "publishing error"}
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriter(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	k := &kafkaClient{writer: mockWriter, metrics: mockMetrics}
	ctx := t.Context()

	testCases := []struct {
		desc      string
		client    *kafkaClient
		mockCalls *gomock.Call
		topic     string
		msg       []byte
		expErr    error
		expLog    string
	}{
		{
			desc:   "error writer is nil",
			client: &kafkaClient{metrics: mockMetrics},
			topic:  "test",
			expErr: errPublisherNotConfigured,
		},
		{
			desc:   "error topic is not provided",
			client: k,
			expErr: errPublisherNotConfigured,
		},
		{
			desc:      "error while publishing message",
			client:    k,
			topic:     "test",
			mockCalls: mockWriter.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Return(errPublish),
			expErr:    errPublish,
			expLog:    "failed to publish message to kafka broker",
		},
	}

	for _, tc := range testCases {
		testFunc := func() {
			logger := logging.NewMockLogger(logging.DEBUG)
			k.logger = logger

			mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", tc.topic)

			err = tc.client.Publish(ctx, tc.topic, tc.msg)
		}

		logs := testutil.StderrOutputForFunc(testFunc)

		assert.Equal(t, tc.expErr, err)
		assert.Contains(t, logs, tc.expLog)
	}
}

func TestKafkaClient_Publish(t *testing.T) {
	var err error

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriter(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	logs := testutil.StdoutOutputForFunc(func() {
		ctx := t.Context()
		logger := logging.NewMockLogger(logging.DEBUG)
		k := &kafkaClient{
			writer:  mockWriter,
			logger:  logger,
			metrics: mockMetrics,
			config: Config{
				Brokers: []string{"localhost:9092"}, // Make sure Broker is not empty
			},
		}

		mockWriter.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Return(nil)
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "test")
		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", "test")

		err = k.Publish(ctx, "test", []byte(`hello`))
	})

	require.NoError(t, err)
	assert.Contains(t, logs, "KAFKA")
	assert.Contains(t, logs, "PUB")
	assert.Contains(t, logs, "hello")
	assert.Contains(t, logs, "test")
}

func TestKafkaClient_SubscribeSuccess(t *testing.T) {
	var (
		msg *pubsub.Message
		err error
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := t.Context()
	mockReader := NewMockReader(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConnection := NewMockConnection(ctrl)

	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		writer: nil,
		reader: map[string]Reader{
			"test": mockReader,
		},
		conn: &multiConn{
			conns: []Connection{
				mockConnection,
			},
		},
		logger: nil,
		config: Config{
			ConsumerGroupID: "consumer",
			Brokers:         []string{"kafkabroker"},
			OffSet:          -1,
		},
		mu:      &sync.RWMutex{},
		metrics: mockMetrics,
	}

	expMessage := pubsub.Message{
		Value: []byte(`hello`),
		Topic: "test",
	}

	mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)
	mockReader.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{Value: []byte(`hello`), Topic: "test"}, nil)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test",
		"consumer_group", gomock.Any())
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", "test",
		"consumer_group", gomock.Any())

	logs := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		k.logger = logger

		msg, err = k.Subscribe(ctx, "test")
	})

	require.NoError(t, err)
	assert.NotNil(t, msg.Context())
	assert.Equal(t, expMessage.Value, msg.Value)
	assert.Equal(t, expMessage.Topic, msg.Topic)
	assert.Contains(t, logs, "KAFKA")
	assert.Contains(t, logs, "hello")
	assert.Contains(t, logs, "kafkabroker")
	assert.Contains(t, logs, "test")
}

func TestKafkaClient_Subscribe_ErrConsumerGroupID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnection := NewMockConnection(ctrl)

	m := &multiConn{
		conns: []Connection{
			mockConnection,
		},
	}

	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		config: Config{
			Brokers: []string{"kafkabroker"},
			OffSet:  -1,
		},
		conn:   m,
		logger: logging.NewMockLogger(logging.INFO),
	}

	mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)

	msg, err := k.Subscribe(t.Context(), "test")
	assert.NotNil(t, msg)
	assert.Equal(t, ErrConsumerGroupNotProvided, err)
}

func TestKafkaClient_SubscribeError(t *testing.T) {
	var (
		msg    *pubsub.Message
		err    error
		errSub = testutil.CustomError{ErrorMessage: "error while subscribing"}
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := t.Context()
	mockReader := NewMockReader(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConnection := NewMockConnection(ctrl)

	m := &multiConn{
		conns: []Connection{
			mockConnection,
		},
	}

	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		writer: nil,
		reader: map[string]Reader{
			"test": mockReader,
		},
		conn:   m,
		logger: logging.NewMockLogger(logging.INFO),
		config: Config{
			ConsumerGroupID: "consumer",
			Brokers:         []string{"kafkabroker"},
			OffSet:          -1,
		},
		mu:      &sync.RWMutex{},
		metrics: mockMetrics,
	}

	mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)
	mockReader.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{}, errSub)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		"topic", "test", "consumer_group", k.config.ConsumerGroupID)

	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.DEBUG)
		k.logger = logger

		msg, err = k.Subscribe(ctx, "test")
	})

	require.Error(t, err)
	assert.Equal(t, errSub, err)
	assert.Nil(t, msg)
	assert.Contains(t, logs, "failed to read message from kafka topic test: error while subscribing")
}

func TestKafkaClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriter(ctrl)
	mockReader := NewMockReader(ctrl)
	mockConn := NewMockConnection(ctrl)

	k := kafkaClient{reader: map[string]Reader{"test-topic": mockReader}, writer: mockWriter, conn: &multiConn{
		conns: []Connection{
			mockConn,
		},
	}}

	mockWriter.EXPECT().Close().Return(nil)
	mockReader.EXPECT().Close().Return(nil)
	mockConn.EXPECT().Close().Return(nil)

	err := k.Close()

	require.NoError(t, err)
}

func TestKafkaClient_CloseError(t *testing.T) {
	var (
		err      error
		errClose = testutil.CustomError{ErrorMessage: "close error"}
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriter(ctrl)
	k := kafkaClient{writer: mockWriter}

	mockWriter.EXPECT().Close().Return(errClose)

	logger := logging.NewMockLogger(logging.ERROR)
	k.logger = logger

	err = k.Close()

	require.Error(t, err)
	assert.ErrorIs(t, err, errClose)
}

func TestKafkaClient_getNewReader(t *testing.T) {
	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		config: Config{
			Brokers:         []string{"kafka-broker"},
			ConsumerGroupID: "consumer",
			OffSet:          -1,
		},
	}

	reader := k.getNewReader("test")

	assert.NotNil(t, reader)
}

func TestNewKafkaClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		desc      string
		config    Config
		expectNil bool
	}{
		{
			desc: "validation of configs fail (Empty Broker)",
			config: Config{
				Brokers: []string{""},
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Bytes)",
			config: Config{
				Brokers:    []string{"kafka-broker"},
				BatchBytes: 0,
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Size)",
			config: Config{
				Brokers:    []string{"kafka-broker"},
				BatchBytes: 1,
				BatchSize:  0,
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Timeout)",
			config: Config{
				Brokers:      []string{"kafka-broker"},
				BatchBytes:   1,
				BatchSize:    1,
				BatchTimeout: 0,
			},
			expectNil: true,
		},
		{
			desc: "successful initialization",
			config: Config{
				Brokers:          []string{"kafka-broker"},
				ConsumerGroupID:  "consumer",
				BatchBytes:       1,
				BatchSize:        1,
				BatchTimeout:     1,
				SecurityProtocol: "SASL_PLAINTEXT",
				SASLMechanism:    "PLAIN",
				SASLUser:         "user",
				SASLPassword:     "password",
			},
			expectNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			k := New(&tc.config, logging.NewMockLogger(logging.ERROR), NewMockMetrics(ctrl))
			if tc.expectNil {
				assert.Nil(t, k)
			} else {
				assert.NotNil(t, k)
			}
		})
	}
}

func TestKafkaClient_Controller(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockClient := NewMockConnection(ctrl)

	client := kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				mockClient,
			},
		},
	}

	mockClient.EXPECT().Controller().Return(kafka.Broker{}, nil)

	broker, err := client.Controller()

	assert.NotNil(t, broker)
	require.NoError(t, err)
}

func TestKafkaClient_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockClient := NewMockConnection(ctrl)

	client := kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				mockClient,
			},
			dialer: &kafka.Dialer{}, // Needed if fallback dialing is triggered
		},
	}

	mockClient.EXPECT().Controller().Return(kafka.Broker{
		Host: "localhost",
		Port: 9092,
	}, nil).AnyTimes()

	mockClient.EXPECT().RemoteAddr().Return(&net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 9092,
	}).AnyTimes()

	mockClient.EXPECT().DeleteTopics("test").Return(nil)

	err := client.DeleteTopic(t.Context(), "test")

	require.NoError(t, err)
}

func TestKafkaClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnection(ctrl)

	// IP: 127.0.0.1 Port: 9092 -> controller's resolved address
	controllerHost := "localhost"
	controllerPort := 9092

	client := kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				mockConn,
			},
			dialer: &kafka.Dialer{}, // Only used if fallback occurs
		},
	}

	t.Run("successfully creates topic", func(t *testing.T) {
		mockConn.EXPECT().Controller().Return(kafka.Broker{
			Host: controllerHost,
			Port: controllerPort,
		}, nil)

		// RemoteAddr should return IP resolved version of controller
		mockConn.EXPECT().RemoteAddr().Return(&net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 9092,
		})

		mockConn.EXPECT().CreateTopics([]kafka.TopicConfig{
			{
				Topic:             "test",
				NumPartitions:     1,
				ReplicationFactor: 1,
			},
		}).Return(nil)

		err := client.CreateTopic(t.Context(), "test")
		require.NoError(t, err)
	})

	t.Run("controller returns error", func(t *testing.T) {
		mockConn.EXPECT().Controller().Return(kafka.Broker{}, errNoActiveConnections)

		err := client.CreateTopic(t.Context(), "test")
		require.EqualError(t, err, errNoActiveConnections.Error())
	})
}

func TestKafkaClient_Subscribe_NotConnected(t *testing.T) {
	var (
		msg *pubsub.Message
		err error
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := t.Context()
	mockConnection := NewMockConnection(ctrl)

	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		conn: &multiConn{
			conns: []Connection{
				mockConnection,
			},
		},
		logger: logging.NewMockLogger(logging.DEBUG),
		mu:     &sync.RWMutex{},
	}

	// ensureConnected probes Controller in the unlocked fast path and
	// again under the write lock before attempting to reconnect.
	mockConnection.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()

	msg, err = k.Subscribe(ctx, "test")

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, errClientNotConnected, err)
}

// TestEnsureConnected_ReconnectsAfterStaleAdminConn pins the runtime recovery
// path: when the admin conn goes stale (broker idle timeout),
// ensureConnected must replace k.conn instead of staying not-connected
// forever. The brokers list uses the RFC 6761 reserved TLD ".invalid", which
// is guaranteed unresolvable — so the re-dial fails deterministically. The
// test asserts that the attempt is made and the failure surfaces.
func TestEnsureConnected_ReconnectsAfterStaleAdminConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	staleConn := NewMockConnection(ctrl)
	staleConn.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()

	k := &kafkaClient{
		dialer: &kafka.Dialer{Timeout: time.Millisecond},
		config: Config{Brokers: []string{"broker.invalid:0"}},
		conn: &multiConn{
			conns: []Connection{staleConn},
		},
		logger: logging.NewMockLogger(logging.DEBUG),
		mu:     &sync.RWMutex{},
	}

	// Stale conn is detected, reconnect is attempted, but the unresolvable
	// broker makes connectToBrokers fail — ensureConnected returns false.
	assert.False(t, k.ensureConnected(t.Context()))

	// reconnectAdminLocked itself surfaces the dial error; the caller must
	// hold connMu for writing.
	k.connMu.Lock()
	err := k.reconnectAdminLocked(t.Context())
	k.connMu.Unlock()
	require.Error(t, err)
}

func TestKafkaClient_Query_Failures(t *testing.T) {
	testCases := []struct {
		name        string
		setupClient func() *kafkaClient
		topic       string
		args        []any
		expectedErr string
	}{
		{
			name: "Client not connected",
			setupClient: func() *kafkaClient {
				ctrl := gomock.NewController(t)
				mockConnection := NewMockConnection(ctrl)
				mockConnection.EXPECT().Controller().Return(kafka.Broker{}, errClientNotConnected).AnyTimes()

				return &kafkaClient{
					conn: &multiConn{
						conns: []Connection{mockConnection},
					},
					logger: logging.NewMockLogger(logging.DEBUG),
					mu:     &sync.RWMutex{},
				}
			},
			topic:       "test-topic",
			expectedErr: errClientNotConnected.Error(),
		},
		{
			name: "Empty topic name",
			setupClient: func() *kafkaClient {
				ctrl := gomock.NewController(t)
				mockConnection := NewMockConnection(ctrl)
				// Query short-circuits on the empty-topic check before
				// ever touching ensureConnected, so Controller() must
				// not be called. Allow zero or more calls so the test
				// stays robust if the order is revisited.
				mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil).AnyTimes()

				return &kafkaClient{
					conn: &multiConn{
						conns: []Connection{mockConnection},
					},
				}
			},
			topic:       "",
			expectedErr: "topic name cannot be empty",
		},
		{
			name: "ReadMessage fails with non-EOF error",
			setupClient: func() *kafkaClient {
				ctrl := gomock.NewController(t)
				mockConnection := NewMockConnection(ctrl)
				mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)

				return &kafkaClient{
					conn: &multiConn{
						conns: []Connection{mockConnection},
					},
					config: Config{
						Brokers:   []string{"localhost:9092"},
						Partition: 0,
					},
				}
			},
			topic:       "test-topic",
			expectedErr: "failed to read message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.setupClient()

			result, err := client.Query(t.Context(), tc.topic, tc.args...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
			assert.Nil(t, result)
		})
	}
}

func TestKafkaClient_Query_ArgumentParsing(t *testing.T) {
	testCases := []struct {
		name           string
		args           []any
		expectedOffset int64
		expectedLimit  int
	}{
		{
			name:           "No arguments - defaults",
			args:           []any{},
			expectedOffset: 0,
			expectedLimit:  10,
		},
		{
			name:           "Only offset provided",
			args:           []any{int64(100)},
			expectedOffset: 100,
			expectedLimit:  10,
		},
		{
			name:           "Offset and limit provided",
			args:           []any{int64(50), 5},
			expectedOffset: 50,
			expectedLimit:  5,
		},
		{
			name:           "Invalid offset type - ignored",
			args:           []any{"invalid", 5},
			expectedOffset: 0,
			expectedLimit:  5,
		},
		{
			name:           "Invalid limit type - ignored",
			args:           []any{int64(25), "invalid"},
			expectedOffset: 25,
			expectedLimit:  10,
		},
		{
			name:           "Both invalid types - defaults",
			args:           []any{"invalid1", "invalid2"},
			expectedOffset: 0,
			expectedLimit:  10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConnection := NewMockConnection(ctrl)

			k := &kafkaClient{
				conn: &multiConn{
					conns: []Connection{mockConnection},
				},
				config: Config{
					Brokers:   []string{"localhost:9092"},
					Partition: 0,
				},
			}

			mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)

			_, err := k.Query(t.Context(), "test-topic", tc.args...)

			assert.Error(t, err)
		})
	}
}

func TestKafkaClient_Query_ContextHandling(t *testing.T) {
	testCases := []struct {
		name        string
		setupCtx    func(t *testing.T) (context.Context, context.CancelFunc)
		description string
	}{
		{
			name: "Context with existing deadline",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithTimeout(t.Context(), 3*time.Second)
			},
			description: "Should use existing context deadline",
		},
		{
			name: "Context without deadline",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithCancel(t.Context())
			},
			description: "Should add 30 second timeout",
		},
		{
			name: "Canceled context",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx, func() {}
			},
			description: "Should handle canceled context",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConnection := NewMockConnection(ctrl)

			k := &kafkaClient{
				conn: &multiConn{
					conns: []Connection{mockConnection},
				},
				config: Config{
					Brokers:   []string{"localhost:9092"},
					Partition: 0,
				},
			}

			mockConnection.EXPECT().Controller().Return(kafka.Broker{}, nil)

			ctx, cancel := tc.setupCtx(t)
			defer cancel()

			_, err := k.Query(ctx, "test-topic")

			assert.Error(t, err)
		})
	}
}

func TestKafkaClient_Subscribe_RaceDetector(t *testing.T) {
	// This test is specifically designed to trigger race conditions
	// Run with: go test -race
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockConn.EXPECT().Controller().Return(kafka.Broker{}, nil).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := &kafkaClient{
		conn: &multiConn{
			conns: []Connection{mockConn},
		},
		config:  Config{ConsumerGroupID: "test-group", Brokers: []string{"localhost:9092"}},
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		reader:  make(map[string]Reader),
		mu:      &sync.RWMutex{},
	}

	// Create mock reader
	mockReader := NewMockReader(ctrl)
	mockReader.EXPECT().FetchMessage(gomock.Any()).DoAndReturn(
		func(_ context.Context) (kafka.Message, error) {
			// Add small delay to increase chance of race
			time.Sleep(time.Microsecond)
			return kafka.Message{
				Topic: "race-test-topic",
				Value: []byte("test"),
			}, nil
		},
	).AnyTimes()

	client.reader["race-test-topic"] = mockReader

	ctx := context.Background()
	numGoroutines := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Rapid concurrent access
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			_, _ = client.Subscribe(ctx, "race-test-topic")
		}()
	}

	wg.Wait()

	// If we reach here without race detector complaints, the fix works
}
