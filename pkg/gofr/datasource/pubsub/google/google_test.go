package google

import (
	"context"
	"errors"
	"testing"
	"time"

	gcPubSub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errTopicExists = errors.New("topic already exists")

	errTestSentinel = errors.New("test-error")
)

func getGoogleClient(t *testing.T) *gcPubSub.Client {
	t.Helper()

	srv := pstest.NewServer()

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("could not initialize a connection to dummy server")
	}

	client, err := gcPubSub.NewClient(t.Context(), "project", option.WithGRPCConn(conn))
	if err != nil {
		t.Errorf("could not initialize a test client")
	}

	return client
}

func TestGoogleClient_New_InvalidConfig(t *testing.T) {
	var g *googleClient

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	out := testutil.StderrOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.ERROR)

		g = New(Config{}, logger, NewMockMetrics(ctrl))
	})

	assert.Nil(t, g)

	assert.Contains(t, out, "could not configure google pubsub")
}

func TestGoogleClient_New_EmptyClient(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	logger := logging.NewMockLogger(logging.DEBUG)

	config := Config{ProjectID: "test", SubscriptionName: "test"}

	client := New(config, logger, mockMetrics)

	require.Nil(t, client.client, "TestGoogleClient_New_EmptyClient Failed!")

	require.Equal(t, config, client.Config, "TestGoogleClient_New_EmptyClient Failed!")
}

func TestGoogleClient_Publish_Success(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-topic"

	message := []byte("test message")

	out := testutil.StdoutOutputForFunc(func() {
		g := &googleClient{
			logger: logging.NewMockLogger(logging.DEBUG),
			client: client,
			Config: Config{
				ProjectID:        "test",
				SubscriptionName: "sub",
			},
			metrics: mockMetrics,
		}

		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", topic)

		mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", topic)

		err := g.Publish(t.Context(), topic, message)

		require.NoError(t, err)
	})

	assert.Contains(t, out, "PUB")
	assert.Contains(t, out, "test message")
	assert.Contains(t, out, "test-topic")
	assert.Contains(t, out, "test")
	assert.Contains(t, out, "GCP")
}

func TestGoogleClient_PublishTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}, metrics: mockMetrics, logger: logging.NewMockLogger(logging.DEBUG)}

	defer g.client.Close()

	ctx, cancel := context.WithCancel(t.Context())

	cancel()

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "test-topic")

	err := g.Publish(ctx, "test-topic", []byte(""))

	require.ErrorContains(t, err, "context canceled")
}

func TestGoogleClient_getTopic_Success(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}

	defer g.client.Close()

	topic, err := g.getTopic(t.Context(), "test-topic")

	require.NoError(t, err)

	assert.Equal(t, "test-topic", topic.ID())
}

func TestGoogleClient_getTopic_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	cancel()

	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}

	defer g.client.Close()

	topic, err := g.getTopic(ctx, "test-topic")

	assert.Nil(t, topic)

	require.ErrorContains(t, err, "context canceled")
}

func TestGoogleClient_getSubscription(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}

	defer g.client.Close()

	topic, _ := g.client.CreateTopic(t.Context(), "test-topic")

	sub, err := g.getSubscription(t.Context(), topic)

	require.NoError(t, err)

	assert.NotNil(t, sub)
}

func Test_validateConfigs(t *testing.T) {
	testCases := []struct {
		desc   string
		input  *Config
		expErr error
	}{
		{desc: "project id not provided", input: &Config{}, expErr: errProjectIDNotProvided},
		{desc: "subscription not provided", input: &Config{ProjectID: "test"}, expErr: errSubscriptionNotProvided},
		{desc: "success", input: &Config{ProjectID: "test", SubscriptionName: "subs"}, expErr: nil},
	}

	for _, tc := range testCases {
		err := validateConfigs(tc.input)
		require.ErrorIs(t, err, tc.expErr)
	}
}

func TestGoogleClient_CloseReturnsError(t *testing.T) {
	g := &googleClient{
		client:      getGoogleClient(t),
		receiveChan: make(map[string]chan *pubsub.Message),
	}

	err := g.Close()

	require.NoError(t, err)

	// client empty
	g = &googleClient{receiveChan: make(map[string]chan *pubsub.Message)}

	err = g.Close()

	require.NoError(t, err)
}

func TestGoogleClient_CreateTopic_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	g := &googleClient{client: mockClient, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	tests := []struct {
		name         string
		topicName    string
		mockBehavior func()
		expectedErr  error
	}{
		{
			name:      "CreateTopic_Success",
			topicName: "test-topic",
			mockBehavior: func() {
				mockClient.EXPECT().CreateTopic(t.Context(), "test-topic").Return(&gcPubSub.Topic{}, nil)
			},
			expectedErr: nil,
		},
		{
			name:      "CreateTopic_AlreadyExists",
			topicName: "test-topic",
			mockBehavior: func() {
				mockClient.EXPECT().CreateTopic(t.Context(), "test-topic").Return(&gcPubSub.Topic{}, errTopicExists)
			},
			expectedErr: errTopicExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()
			err := g.CreateTopic(t.Context(), tt.topicName)
			require.ErrorIs(t, err, tt.expectedErr, "expected no error, but got one")
		})
	}
}

func TestGoogleClient_CreateTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	g := &googleClient{client: mockClient, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	mockClient.EXPECT().CreateTopic(t.Context(), "test-topic").
		Return(&gcPubSub.Topic{}, errTestSentinel)

	err := g.CreateTopic(t.Context(), "test-topic")

	require.ErrorIs(t, err, errTestSentinel, "expected test-error but got different error")
}

func TestGoogleClient_CreateTopic_EmptyClient(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	g := &googleClient{client: nil, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	err := g.CreateTopic(t.Context(), "test-topic")

	require.ErrorIs(t, err, errClientNotConnected, "expected client-error but got different error")
}

func TestGoogleClient_DeleteTopic(t *testing.T) {
	ctx := t.Context()

	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{client: client, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	// Test successful topic creation
	t.Run("DeleteTopic_Success", func(t *testing.T) {
		err := g.CreateTopic(ctx, "test-topic")
		require.NoError(t, err)

		err = g.DeleteTopic(ctx, "test-topic")
		require.NoError(t, err, "expected topic deletion to succeed, but got error")
	})

	// Test topic deletion with topic not found
	t.Run("DeleteTopic_NotFound", func(t *testing.T) {
		err := g.DeleteTopic(ctx, "test-topic")
		require.ErrorContains(t, err, "NotFound", "expected NotFound error for non existing topic deletion")
	})
}

func TestGoogleClient_DeleteTopic_EmptyClient(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	g := &googleClient{client: nil, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	err := g.DeleteTopic(t.Context(), "test-topic")

	require.ErrorIs(t, err, errClientNotConnected, "expected client-error but got different error")
}

func TestGoogleClient_Query(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	logger := logging.NewMockLogger(logging.DEBUG)

	topic := "test-topic-query"

	message := []byte("test message")

	g := &googleClient{
		client:  client,
		logger:  logger,
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	topicObj, err := client.CreateTopic(t.Context(), topic)

	require.NoError(t, err)

	subName := "sub-query-" + topic

	_, err = client.CreateSubscription(t.Context(), subName, gcPubSub.SubscriptionConfig{
		Topic: topicObj,
	})

	require.NoError(t, err)

	result := topicObj.Publish(t.Context(), &gcPubSub.Message{Data: message})

	_, err = result.Get(t.Context())

	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)

	defer cancel()

	queryResult, err := g.Query(ctx, topic)

	require.NoError(t, err)

	assert.Equal(t, message, queryResult)

	err = topicObj.Delete(t.Context())

	require.NoError(t, err)
}

func TestIsConnected_WhenClientNotNil(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t)}
	require.True(t, g.isConnected())
}

func TestClose_ClientNil(t *testing.T) {
	g := &googleClient{
		receiveChan: map[string]chan *pubsub.Message{
			"test-topic": make(chan *pubsub.Message),
		},
	}

	err := g.Close()
	require.NoError(t, err)
}

func TestClose_MultipleReceiveChans_ClientNil(t *testing.T) {
	g := &googleClient{
		receiveChan: map[string]chan *pubsub.Message{
			"topic1": make(chan *pubsub.Message),
			"topic2": make(chan *pubsub.Message),
		},
		// client is nil
	}

	err := g.Close()
	require.NoError(t, err)
}

func TestSubscribe_ClientNil(t *testing.T) {
	g := &googleClient{}

	msg, err := g.Subscribe(t.Context(), "test-topic")
	require.Nil(t, msg)
	require.NoError(t, err)
}

func TestGetTopic_ClientNil(t *testing.T) {
	g := &googleClient{}

	_, err := g.getTopic(t.Context(), "any-topic")
	require.Equal(t, errClientNotConnected, err)
}

func TestIsConnected_WhenClientNil(t *testing.T) {
	g := &googleClient{}
	require.False(t, g.isConnected())
}

func TestGoogleClient_getTopic_CreateFailure(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	// Delete the server to simulate failure in CreateTopic
	client.Close()

	g := &googleClient{client: client, Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}

	_, err := g.getTopic(t.Context(), "test-topic")

	require.Error(t, err)
}

func TestGoogleClient_collectMessages_LimitReached(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	g := &googleClient{
		logger: logger,
	}

	msgChan := make(chan []byte, 3)

	msgChan <- []byte("message1")

	msgChan <- []byte("message2")

	close(msgChan)

	ctx := t.Context()

	result := g.collectMessages(ctx, msgChan, 2)

	expected := []byte("message1\nmessage2")

	assert.Equal(t, expected, result)
}

func TestGoogleClient_getQuerySubscription_CreateFails(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
	}

	topic, err := client.CreateTopic(t.Context(), "test-topic-bad")

	require.NoError(t, err)

	// simulate failure by closing client
	client.Close()

	sub, err := g.getQuerySubscription(t.Context(), topic)

	require.Error(t, err)

	assert.Nil(t, sub)
}
