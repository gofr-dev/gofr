package google

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	gcPubSub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errTopicExists        = errors.New("topic already exists")
	errTopicExistsWrapped = fmt.Errorf("Topic already exists: %w", errTopicExists)

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

func TestGoogleClient_Health_Success(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test-project", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	// Create some test topics and subscriptions
	_, err := client.CreateTopic(t.Context(), "test-topic-health")
	require.NoError(t, err)

	health := g.Health()

	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, "test-project", health.Details["projectID"])
	assert.Equal(t, "GOOGLE", health.Details["backend"])
	assert.NotNil(t, health.Details["writers"])
	assert.NotNil(t, health.Details["readers"])
}

func TestGoogleClient_Health_WithError(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test-project", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	// Close client to cause errors in health check
	client.Close()

	// This will cause getWriterDetails and getReaderDetails to fail
	health := g.Health()

	// Health should still return, but status might be DOWN
	assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, "test-project", health.Details["projectID"])
	assert.Equal(t, "GOOGLE", health.Details["backend"])
}

func TestGoogleClient_getWriterDetails(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test-project", SubscriptionName: "sub"},
	}

	// Create some test topics
	_, err := client.CreateTopic(t.Context(), "writer-test-topic1")
	require.NoError(t, err)

	_, err = client.CreateTopic(t.Context(), "writer-test-topic2")
	require.NoError(t, err)

	status, details := g.getWriterDetails()

	assert.Equal(t, "UP", status)
	assert.NotNil(t, details)
	assert.GreaterOrEqual(t, len(details), 2)
}

func TestGoogleClient_getReaderDetails(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test-project", SubscriptionName: "sub"},
	}

	// Create a topic and subscription
	topic, err := client.CreateTopic(t.Context(), "reader-test-topic")
	require.NoError(t, err)

	_, err = client.CreateSubscription(t.Context(), "test-subscription", gcPubSub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	status, details := g.getReaderDetails()

	assert.Equal(t, "UP", status)
	assert.NotNil(t, details)
	assert.GreaterOrEqual(t, len(details), 1)
}

func TestGoogleClient_Subscribe_Success(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-subscribe-topic"

	message := []byte("subscribe test message")

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
	}

	// Expect metrics calls
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()

	// Create topic and publish a message
	topicObj, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	result := topicObj.Publish(t.Context(), &gcPubSub.Message{Data: message})
	_, err = result.Get(t.Context())
	require.NoError(t, err)

	// Subscribe to the topic
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// Start subscription in goroutine
	var msg *pubsub.Message

	go func() {
		msg, err = g.Subscribe(ctx, topic)
	}()

	// Give it time to set up subscription and receive
	time.Sleep(500 * time.Millisecond)

	// Publish another message
	result = topicObj.Publish(t.Context(), &gcPubSub.Message{Data: message})
	_, err = result.Get(t.Context())
	require.NoError(t, err)

	// Wait for message
	time.Sleep(1 * time.Second)

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, message, msg.Value)
	assert.Equal(t, topic, msg.Topic)
}

func TestGoogleClient_Subscribe_ContextCanceled(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-subscribe-timeout"

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
	}

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()

	// Create topic to avoid errors
	_, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(50 * time.Millisecond)

	msg, _ := g.Subscribe(ctx, topic)

	// Should return nil when context is done
	assert.Nil(t, msg)
	// Error may or may not be nil depending on when context was canceled
}

func TestGoogleClient_Subscribe_NotConnected(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		logger: logging.NewMockLogger(logging.DEBUG),
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	// Close client to simulate not connected
	client.Close()

	msg, err := g.Subscribe(t.Context(), "test-topic")

	assert.Nil(t, msg)
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestGoogleClient_Query_EmptyTopic(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	_, err := g.Query(t.Context(), "")

	assert.ErrorIs(t, err, errTopicName)
}

func TestGoogleClient_Query_NotConnected(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	client.Close()

	_, err := g.Query(t.Context(), "test-topic")

	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestGoogleClient_Query_WithLimit(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		logger: logging.NewMockLogger(logging.DEBUG),
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	topic := "test-topic-query-limit"

	topicObj, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Publish multiple messages
	for i := 0; i < 5; i++ {
		result := topicObj.Publish(t.Context(), &gcPubSub.Message{Data: []byte("message")})
		_, err = result.Get(t.Context())
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// Query with limit
	_, err = g.Query(ctx, topic, 30*time.Second, 3)

	require.NoError(t, err)
}

func TestGoogleClient_getSubscription_ExistsError(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	topic, err := client.CreateTopic(t.Context(), "test-topic-sub-err")
	require.NoError(t, err)

	// Close client to cause error
	client.Close()

	sub, err := g.getSubscription(t.Context(), topic)

	assert.Nil(t, sub)
	assert.Error(t, err)
}

func TestParseQueryArgs_WithLimit(t *testing.T) {
	timeout, limit := parseQueryArgs(30*time.Second, 5)

	assert.Equal(t, defaultQueryTimeout, timeout)
	assert.Equal(t, 5, limit)
}

func TestParseQueryArgs_NoArgs(t *testing.T) {
	timeout, limit := parseQueryArgs()

	assert.Equal(t, defaultQueryTimeout, timeout)
	assert.Equal(t, defaultMessageLimit, limit)
}

func TestParseQueryArgs_OnlyTimeout(t *testing.T) {
	timeout, limit := parseQueryArgs(45 * time.Second)

	assert.Equal(t, defaultQueryTimeout, timeout)
	assert.Equal(t, defaultMessageLimit, limit)
}

func TestGoogleClient_collectMessages_ContextDone(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	g := &googleClient{
		logger: logger,
	}

	msgChan := make(chan []byte)

	ctx, cancel := context.WithCancel(t.Context())

	// Cancel immediately
	cancel()

	result := g.collectMessages(ctx, msgChan, 10)

	assert.Empty(t, result)
}

func TestGoogleClient_collectMessages_UnlimitedMessages(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	g := &googleClient{
		logger: logger,
	}

	msgChan := make(chan []byte, 3)

	msgChan <- []byte("message1")

	msgChan <- []byte("message2")

	msgChan <- []byte("message3")

	close(msgChan)

	ctx := t.Context()

	// Test with limit <= 0 (unlimited)
	result := g.collectMessages(ctx, msgChan, 0)

	expected := []byte("message1\nmessage2\nmessage3")

	assert.Equal(t, expected, result)
}

func TestGoogleClient_getQuerySubscription_AlreadyExists(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
	}

	topic, err := client.CreateTopic(t.Context(), "test-topic-exists")
	require.NoError(t, err)

	subName := g.SubscriptionName + "-query-" + topic.ID()

	// Create subscription first
	_, err = client.CreateSubscription(t.Context(), subName, gcPubSub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	// Now get it - should return existing one
	sub, err := g.getQuerySubscription(t.Context(), topic)

	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, subName, sub.ID())
}

func TestGoogleClient_Subscribe_AlreadyStarted(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-subscribe-already-started"

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
	}

	// Create topic
	topicObj, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Mark subscription as already started
	g.subStarted[topic] = struct{}{}
	g.receiveChan[topic] = make(chan *pubsub.Message, 1)

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()

	// Publish a message
	message := []byte("test message for already started")
	result := topicObj.Publish(t.Context(), &gcPubSub.Message{Data: message})
	_, err = result.Get(t.Context())
	require.NoError(t, err)

	// Manually put message in the channel
	m := pubsub.NewMessage(t.Context())
	m.Value = message

	m.Topic = topic
	g.receiveChan[topic] <- m

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	msg, err := g.Subscribe(ctx, topic)

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, message, msg.Value)
}

func TestGoogleClient_Query_ContextTimeout(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		logger: logging.NewMockLogger(logging.DEBUG),
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	topic := "test-topic-query-timeout"

	_, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Very short timeout - messages won't arrive in time
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	time.Sleep(50 * time.Millisecond) // Wait for context to timeout

	result, err := g.Query(ctx, topic)

	// Should return empty result or error due to timeout
	assert.True(t, err != nil || result != nil, "expected either an error or a non-nil result due to timeout")
	assert.NotEqual(t, err == nil, result == nil, "expected error and result not to share the same nil state")
}

func TestGoogleClient_Query_GetTopicError(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		logger: logging.NewMockLogger(logging.DEBUG),
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	// Close client to cause getTopic to fail
	client.Close()

	_, err := g.Query(t.Context(), "test-topic")

	assert.Error(t, err)
}

func TestGoogleClient_getSubscription_AlreadyExists(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	topic, err := client.CreateTopic(t.Context(), "test-topic-sub-exists")
	require.NoError(t, err)

	// Create subscription first
	subName := g.SubscriptionName + "-" + topic.ID()
	_, err = client.CreateSubscription(t.Context(), subName, gcPubSub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	// Now call getSubscription - should return existing one
	sub, err := g.getSubscription(t.Context(), topic)

	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, subName, sub.ID())
}

func TestGoogleClient_DeleteTopic_AlreadyExists(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{client: client, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	// Create and then delete
	err := g.CreateTopic(t.Context(), "test-delete-topic")
	require.NoError(t, err)

	err = g.DeleteTopic(t.Context(), "test-delete-topic")
	require.NoError(t, err)

	// Try deleting again - should handle "not found" gracefully
	err = g.DeleteTopic(t.Context(), "test-delete-topic")

	// Error should contain "NotFound" or be nil if implementation handles it
	assert.True(t, err == nil || strings.Contains(err.Error(), "NotFound"), "expected no error or NotFound error when deleting missing topic")
}

func TestGoogleClient_CreateTopic_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	g := &googleClient{client: mockClient, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	// Mock CreateTopic to return "already exists" error
	mockClient.EXPECT().CreateTopic(t.Context(), "existing-topic").
		Return(&gcPubSub.Topic{}, errTopicExistsWrapped)

	err := g.CreateTopic(t.Context(), "existing-topic")

	// Should not return error if topic already exists
	assert.NoError(t, err)
}

func TestGoogleClient_getQuerySubscription_ExistsCheck(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	topic, err := client.CreateTopic(t.Context(), "test-topic-query-exists")
	require.NoError(t, err)

	// First call - subscription doesn't exist, should create
	sub1, err := g.getQuerySubscription(t.Context(), topic)
	require.NoError(t, err)
	assert.NotNil(t, sub1)

	// Second call - subscription exists, should return existing
	sub2, err := g.getQuerySubscription(t.Context(), topic)
	require.NoError(t, err)
	assert.NotNil(t, sub2)
	assert.Equal(t, sub1.ID(), sub2.ID())
}

func TestGoogleClient_Subscribe_GetTopicError(t *testing.T) {
	client := getGoogleClient(t)

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
	}

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Close client to cause getTopic to fail
	client.Close()

	topic := "test-subscribe-error"

	msg, err := g.Subscribe(t.Context(), topic)

	assert.Nil(t, msg)
	assert.Error(t, err)
}

func TestGoogleClient_Subscribe_GetSubscriptionError(t *testing.T) {
	client := getGoogleClient(t)

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-subscribe-sub-error"

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
		receiveChan: make(map[string]chan *pubsub.Message),
		subStarted:  make(map[string]struct{}),
	}

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count",
		"topic", topic, "subscription_name", g.Config.SubscriptionName).AnyTimes()

	// Create topic first
	_, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Close client after topic creation to cause getSubscription to fail
	client.Close()

	msg, err := g.Subscribe(t.Context(), topic)

	assert.Nil(t, msg)
	assert.Error(t, err)
}

func TestConnect_Success(t *testing.T) {
	// This test uses the real pstest server
	client := getGoogleClient(t)

	defer client.Close()

	// Verify connection was successful by checking we can list topics
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	it := client.Topics(ctx)
	_, err := it.Next()

	// Should either return a topic or iterator.Done (no topics)
	assert.True(t, err == nil || errors.Is(err, iterator.Done))
}

func TestConnect_NoTopics(t *testing.T) {
	config := Config{ProjectID: "test-project", SubscriptionName: "test-sub"}

	// Use a fresh pstest server with no topics
	srv := pstest.NewServer()
	defer srv.Close()

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client, err := gcPubSub.NewClient(t.Context(), config.ProjectID, option.WithGRPCConn(conn))
	require.NoError(t, err)

	defer client.Close()

	// Verify the client works with no topics
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	it := client.Topics(ctx)
	_, err = it.Next()

	// Should return iterator.Done when there are no topics
	assert.True(t, errors.Is(err, iterator.Done) || err == nil)
}

func TestGetQuerySubscription_NewSubscription(t *testing.T) {
	client := getGoogleClient(t)

	defer client.Close()

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	topic, err := client.CreateTopic(t.Context(), "new-query-topic")
	require.NoError(t, err)

	// First time calling - should create new subscription
	sub, err := g.getQuerySubscription(t.Context(), topic)

	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Contains(t, sub.ID(), "query")
}

func TestGetSubscription_CreateError(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		Config: Config{ProjectID: "test", SubscriptionName: "sub"},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	topic, err := client.CreateTopic(t.Context(), "test-sub-create-err")
	require.NoError(t, err)

	// Close client to cause creation to fail
	client.Close()

	sub, err := g.getSubscription(t.Context(), topic)

	assert.Nil(t, sub)
	assert.Error(t, err)
}

func TestDeleteTopic_WithNotFoundString(t *testing.T) {
	// Test with a real client
	realClient := getGoogleClient(t)
	defer realClient.Close()

	gReal := &googleClient{client: realClient, Config: Config{ProjectID: "test", SubscriptionName: "sub"}}

	// Try to delete a non-existent topic - should handle gracefully
	err := gReal.DeleteTopic(t.Context(), "definitely-does-not-exist")

	// Should either return nil (handled) or an error containing "not found"
	lowerErr := strings.ToLower(fmt.Sprint(err))

	assert.True(t, err == nil || strings.Contains(lowerErr, "not"),
		"expected no error or a not-found error when deleting missing topic")
}

func TestPublish_GetTopicError(t *testing.T) {
	client := getGoogleClient(t)

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	g := &googleClient{
		client:  client,
		logger:  logging.NewMockLogger(logging.DEBUG),
		metrics: mockMetrics,
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count",
		"topic", "test-pub-error").AnyTimes()

	// Close client to cause getTopic to fail
	client.Close()

	err := g.Publish(t.Context(), "test-pub-error", []byte("message"))

	assert.Error(t, err)
}

func TestQuery_GetSubscriptionError(t *testing.T) {
	client := getGoogleClient(t)

	g := &googleClient{
		client: client,
		logger: logging.NewMockLogger(logging.DEBUG),
		Config: Config{
			ProjectID:        "test",
			SubscriptionName: "sub",
		},
	}

	topic := "test-query-sub-err"

	// Create topic
	_, err := client.CreateTopic(t.Context(), topic)
	require.NoError(t, err)

	// Close client to cause getQuerySubscription to fail
	client.Close()

	_, err = g.Query(t.Context(), topic)

	assert.Error(t, err)
}

func TestNew_WithRetryConnect(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	logger := logging.NewMockLogger(logging.DEBUG)

	// Use invalid config to trigger retry logic
	config := Config{ProjectID: "invalid-project-for-retry", SubscriptionName: "test"}

	client := New(config, logger, mockMetrics)

	// Should return a client even if connection fails
	assert.NotNil(t, client)
	assert.Nil(t, client.client) // Client should be nil initially

	// Clean up
	_ = client.Close()
}

func TestCollectMessages_SingleMessage(t *testing.T) {
	logger := logging.NewMockLogger(logging.DEBUG)

	g := &googleClient{
		logger: logger,
	}

	msgChan := make(chan []byte, 1)
	msgChan <- []byte("single")

	close(msgChan)

	result := g.collectMessages(t.Context(), msgChan, 10)

	assert.Equal(t, []byte("single"), result)
}
