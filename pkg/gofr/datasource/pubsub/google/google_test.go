package google

import (
	"context"
	"testing"

	gcPubSub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func getGoogleClient(t *testing.T) *gcPubSub.Client {
	t.Helper()

	srv := pstest.NewServer()

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		err := g.Publish(context.Background(), topic, message)

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

	ctx, cancel := context.WithCancel(context.Background())

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

	topic, err := g.getTopic(context.Background(), "test-topic")

	require.NoError(t, err)
	assert.Equal(t, "test-topic", topic.ID())
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
	require.ErrorContains(t, err, "context canceled")
}

func TestGoogleClient_getSubscription(t *testing.T) {
	g := &googleClient{client: getGoogleClient(t), Config: Config{
		ProjectID:        "test",
		SubscriptionName: "sub",
	}}
	defer g.client.Close()

	topic, _ := g.client.CreateTopic(context.Background(), "test-topic")

	sub, err := g.getSubscription(context.Background(), topic)

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
	// client already present
	g := &googleClient{
		client: getGoogleClient(t),
	}

	err := g.Close()

	require.NoError(t, err)

	// client empty
	g = &googleClient{}

	err = g.Close()

	require.NoError(t, err)
}
