package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestValidateConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name:     "Valid Config",
			config:   Config{Broker: "kafkabroker", BatchSize: 1, BatchBytes: 1, BatchTimeout: 1},
			expected: nil,
		},
		{
			name:     "Empty Broker",
			config:   Config{BatchSize: 1, BatchBytes: 1, BatchTimeout: 1},
			expected: errBrokerNotProvided,
		},
		{
			name:     "Zero BatchSize",
			config:   Config{Broker: "kafkabroker", BatchSize: 0, BatchBytes: 1, BatchTimeout: 1},
			expected: errBatchSize,
		},
		{
			name:     "Zero BatchBytes",
			config:   Config{Broker: "kafkabroker", BatchSize: 1, BatchBytes: 0, BatchTimeout: 1},
			expected: errBatchBytes,
		},
		{
			name:     "Zero BatchTimeout",
			config:   Config{Broker: "kafkabroker", BatchSize: 1, BatchBytes: 1, BatchTimeout: 0},
			expected: errBatchTimeout,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigs(tc.config)
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
	ctx := context.TODO()

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
		ctx := context.TODO()
		logger := logging.NewMockLogger(logging.DEBUG)
		k := &kafkaClient{writer: mockWriter, logger: logger, metrics: mockMetrics}

		mockWriter.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).
			Return(nil)
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

	ctx := context.TODO()
	mockReader := NewMockReader(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		writer: nil,
		reader: map[string]Reader{
			"test": mockReader,
		},
		logger: nil,
		config: Config{
			ConsumerGroupID: "consumer",
			Broker:          "kafkabroker",
			OffSet:          -1,
		},
		mu:      &sync.RWMutex{},
		metrics: mockMetrics,
	}

	expMessage := pubsub.Message{
		Value: []byte(`hello`),
		Topic: "test",
	}

	mockReader.EXPECT().ReadMessage(gomock.Any()).
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
	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		config: Config{
			Broker: "kafkabroker",
			OffSet: -1,
		},
		logger: logging.NewMockLogger(logging.INFO),
	}

	msg, err := k.Subscribe(context.TODO(), "test")
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

	ctx := context.TODO()
	mockReader := NewMockReader(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		writer: nil,
		reader: map[string]Reader{
			"test": mockReader,
		},
		logger: logging.NewMockLogger(logging.INFO),
		config: Config{
			ConsumerGroupID: "consumer",
			Broker:          "kafkabroker",
			OffSet:          -1,
		},
		mu:      &sync.RWMutex{},
		metrics: mockMetrics,
	}

	mockReader.EXPECT().ReadMessage(gomock.Any()).
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

	k := kafkaClient{reader: map[string]Reader{"test-topic": mockReader}, writer: mockWriter, conn: mockConn}

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
	assert.Equal(t, errClose, err)
}

func TestKafkaClient_getNewReader(t *testing.T) {
	k := &kafkaClient{
		dialer: &kafka.Dialer{},
		config: Config{
			Broker:          "kafka-broker",
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
				Broker: "",
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Bytes)",
			config: Config{
				Broker:     "kafka-broker",
				BatchBytes: 0,
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Size)",
			config: Config{
				Broker:     "kafka-broker",
				BatchBytes: 1,
				BatchSize:  0,
			},
			expectNil: true,
		},
		{
			desc: "validation of configs fail (Zero Batch Timeout)",
			config: Config{
				Broker:       "kafka-broker",
				BatchBytes:   1,
				BatchSize:    1,
				BatchTimeout: 0,
			},
			expectNil: true,
		},
		{
			desc: "successful initialization",
			config: Config{
				Broker:          "kafka-broker",
				ConsumerGroupID: "consumer",
				BatchBytes:      1,
				BatchSize:       1,
				BatchTimeout:    1,
			},
			expectNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			k := New(tc.config, logging.NewMockLogger(logging.ERROR), NewMockMetrics(ctrl))
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
		conn: mockClient,
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
		conn: mockClient,
	}

	mockClient.EXPECT().DeleteTopics("test").Return(nil)

	err := client.DeleteTopic(context.Background(), "test")

	require.NoError(t, err)
}

func TestKafkaClient_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockClient := NewMockConnection(ctrl)

	client := kafkaClient{
		conn: mockClient,
	}

	testCases := []struct {
		desc string
		err  error
	}{
		{"create success", nil},
		{"delete success", testutil.CustomError{ErrorMessage: "custom error"}},
	}

	for _, tc := range testCases {
		mockClient.EXPECT().CreateTopics(gomock.Any()).Return(tc.err)

		err := client.CreateTopic(context.Background(), "test")

		assert.Equal(t, tc.err, err)
	}
}
