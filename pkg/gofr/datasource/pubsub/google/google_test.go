package google

import (
	"context"
	"testing"

	gcPubSub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/pkg/gofr/testutil"
)

func getGoogleClient(t *testing.T) *gcPubSub.Client {
	srv := pstest.NewServer()

	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("could not initialize a connection to dummy server")
	}

	client, err := gcPubSub.NewClient(context.Background(), "project", option.WithGRPCConn(conn))
	if err != nil {
		t.Errorf("could not initialize a test client")
	}

	return client
}

func TestGoogleClient_New_Error(t *testing.T) {
	var (
		g *googleClient
	)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	out := testutil.StderrOutputForFunc(func() {
		logger := testutil.NewMockLogger(testutil.ERRORLOG)

		g = New(Config{}, logger, NewMockMetrics(ctrl))
	})

	assert.Nil(t, g)
	assert.Contains(t, out, "google pubsub could not be configured")
}

func TestGoogleClient_Publish_Success(t *testing.T) {
	client := getGoogleClient(t)
	defer client.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	topic := "test-topic"
	message := []byte("test message")
	expectedLog := "published google message test message on topic test-topic\n"

	out := testutil.StdoutOutputForFunc(func() {
		g := &googleClient{
			logger: testutil.NewMockLogger(testutil.DEBUGLOG),
			client: client,
			Config: Config{
				ProjectID:        "test",
				SubscriptionName: "sub",
			},
			metrics: mockMetrics,
		}

		mockMetrics.EXPECT().IncrementCounter(context.Background(), "app_pubsub_publish_total_count", "topic", topic)
		mockMetrics.EXPECT().IncrementCounter(context.Background(), "app_pubsub_publish_success_count", "topic", topic)

		err := g.Publish(context.Background(), topic, message)

		assert.Nil(t, err)
	})

	assert.Equal(t, expectedLog, out)
}

func TestGoogleClient_PublishTopic_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}, metrics: mockMetrics}
	defer g.client.Close()

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	mockMetrics.EXPECT().IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test-topic")

	err := g.Publish(ctx, "test-topic", []byte(""))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "context canceled")
	}
}

func TestGoogleClient_getTopic_Success(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}
	defer g.client.Close()

	topic, err := g.getTopic(context.Background(), "test-topic")

	assert.Nil(t, err)
	assert.Equal(t, topic.ID(), "test-topic")
}

func TestGoogleClient_getTopic_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}
	defer g.client.Close()

	topic, err := g.getTopic(ctx, "test-topic")

	assert.Nil(t, topic)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestGoogleClient_getSubscription(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}
	defer g.client.Close()

	topic, _ := g.client.CreateTopic(context.Background(), "test-topic")

	sub, err := g.getSubscription(context.Background(), topic)

	assert.Nil(t, err)
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

		assert.Equal(t, tc.expErr, err)
	}
}

func TestGoogleClient_CreateTopicSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockClient(ctrl)

	client := googleClient{
		client: mock,
	}

	mock.EXPECT().CreateTopic(context.Background(), "test-topic").Return(&gcPubSub.Topic{}, nil)

	err := client.CreateTopic(context.Background(), "test-topic")

	assert.Nil(t, err)
}

func TestGoogleClient_CreateTopic_AlreadyExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockClient(ctrl)

	client := googleClient{
		client: mock,
	}

	mock.EXPECT().CreateTopic(context.Background(), "test-topic").Return(&gcPubSub.Topic{},
		testutil.CustomError{ErrorMessage: "Topic already exists"})

	err := client.CreateTopic(context.Background(), "test-topic")

	assert.Nil(t, err)
}

func TestGoogleClient_CreateTopicFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockClient(ctrl)

	client := googleClient{
		client: mock,
	}

	mock.EXPECT().CreateTopic(context.Background(), "test-topic").Return(&gcPubSub.Topic{},
		testutil.CustomError{ErrorMessage: "Unknown Error"})

	err := client.CreateTopic(context.Background(), "test-topic")

	assert.NotNil(t, err)
}
