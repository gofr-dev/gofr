package kafka

import (
	"context"
	"sync"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_validateConfigs(t *testing.T) {
	config := Config{Broker: "kafkabroker", ConsumerGroupID: "1"}

	err := validateConfigs(config)

	assert.Nil(t, err)
}

func Test_validateConfigsErrConsumerGroupNotFound(t *testing.T) {
	config := Config{Broker: "kafkabroker"}

	err := validateConfigs(config)

	assert.Equal(t, errConsumerGroupNotProvided, err)
}

func Test_validateConfigsErrBrokerNotProvided(t *testing.T) {
	config := Config{ConsumerGroupID: "1"}

	err := validateConfigs(config)

	assert.Equal(t, err, errBrokerNotProvided)
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
			mockCalls: mockWriter.EXPECT().WriteMessages(ctx, gomock.Any()).Return(errPublish),
			expErr:    errPublish,
			expLog:    "failed to publish message to kafka broker",
		},
	}

	for _, tc := range testCases {
		testFunc := func() {
			logger := testutil.NewMockLogger(testutil.DEBUGLOG)
			k.logger = logger

			mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", tc.topic)

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
		logger := testutil.NewMockLogger(testutil.DEBUGLOG)
		k := &kafkaClient{writer: mockWriter, logger: logger, metrics: mockMetrics}

		mockWriter.EXPECT().WriteMessages(ctx, gomock.Any()).
			Return(nil)
		mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test")
		mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", "test")

		err = k.Publish(ctx, "test", []byte(`hello`))
	})

	assert.Nil(t, err)
	assert.Contains(t, logs, "published kafka message hello on topic test")
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

	mockReader.EXPECT().ReadMessage(ctx).
		Return(kafka.Message{Value: []byte(`hello`), Topic: "test"}, nil)
	mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", "test")
	mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", "test")

	logs := testutil.StdoutOutputForFunc(func() {
		logger := testutil.NewMockLogger(testutil.DEBUGLOG)
		k.logger = logger

		msg, err = k.Subscribe(ctx, "test")
	})

	assert.Nil(t, err)
	assert.Equal(t, expMessage.Value, msg.Value)
	assert.Equal(t, expMessage.Topic, msg.Topic)
	assert.Contains(t, logs, "received kafka message hello on topic test")
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
		logger: nil,
		config: Config{
			ConsumerGroupID: "consumer",
			Broker:          "kafkabroker",
			OffSet:          -1,
		},
		mu:      &sync.RWMutex{},
		metrics: mockMetrics,
	}

	mockReader.EXPECT().ReadMessage(ctx).
		Return(kafka.Message{}, errSub)
	mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", "test")

	logs := testutil.StderrOutputForFunc(func() {
		logger := testutil.NewMockLogger(testutil.DEBUGLOG)
		k.logger = logger

		msg, err = k.Subscribe(ctx, "test")
	})

	assert.NotNil(t, err)
	assert.Equal(t, errSub, err)
	assert.Nil(t, msg)
	assert.Contains(t, logs, "failed to read message from Kafka topic test: error while subscribing")
}

func TestKafkaClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWriter := NewMockWriter(ctrl)
	k := kafkaClient{writer: mockWriter}

	mockWriter.EXPECT().Close().Return(nil)

	err := k.Close()

	assert.Nil(t, err)
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

	logs := testutil.StderrOutputForFunc(func() {
		logger := testutil.NewMockLogger(testutil.ERRORLOG)
		k.logger = logger

		err = k.Close()
	})

	assert.NotNil(t, err)
	assert.Equal(t, errClose, err)
	assert.Contains(t, logs, "failed to close Kafka writer")
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
		desc     string
		config   Config
		expected bool
	}{
		{
			desc: "validation of configs fail",
			config: Config{
				Broker: "kafka-broker",
			},
			expected: false,
		},
		{
			desc: "successful initialization",
			config: Config{
				Broker:          "kafka-broker",
				ConsumerGroupID: "consumer",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		k := New(tc.config, testutil.NewMockLogger(testutil.ERRORLOG), NewMockMetrics(ctrl))

		if tc.expected {
			assert.NotNil(t, k)
		} else {
			assert.Nil(t, k)
		}
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
	assert.Nil(t, err)
}

func TestKafkaClient_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockClient := NewMockConnection(ctrl)

	client := kafkaClient{
		conn: mockClient,
	}

	mockClient.EXPECT().DeleteTopics("test").Return(nil)

	err := client.DeleteTopic(context.Background(), "test")

	assert.Nil(t, err)
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
