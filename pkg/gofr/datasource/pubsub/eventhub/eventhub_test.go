package eventhub

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/testutil"
)

// errHealthProbe stands in for whatever the Event Hub SDK returns when the probe fails.
var errHealthProbe = errors.New("event hub unreachable")

// errPartitionClientUnavailable is what the fake consumer returns instead of a partition client,
// which is a concrete type it cannot construct.
var errPartitionClientUnavailable = errors.New("partition client unavailable in tests")

func TestConnect(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug("Event Hub connection started using connection string")
	mockLogger.EXPECT().Debug("Event Hub producer client setup success")
	mockLogger.EXPECT().Debug("Event Hub container client setup success")
	mockLogger.EXPECT().Debug("Event Hub blobstore client setup success")
	mockLogger.EXPECT().Debug("Event Hub consumer client setup success")
	mockLogger.EXPECT().Debug("Event Hub processor setup success")
	mockLogger.EXPECT().Debug("Event Hub processor running successfully").AnyTimes()
	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug("Event Hub client initialization complete")

	client.UseLogger(mockLogger)
	client.UseMetrics(NewMockMetrics(ctrl))
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-eventhub"))

	client.Connect()

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Connection Failed")
}

func TestConfigValidation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockLogger := NewMockLogger(ctrl)

	client := New(Config{})

	client.UseLogger(mockLogger)

	mockLogger.EXPECT().Error("eventhubName cannot be an empty")
	mockLogger.EXPECT().Error("connectionString cannot be an empty")
	mockLogger.EXPECT().Error("storageServiceURL cannot be an empty")
	mockLogger.EXPECT().Error("storageContainerName cannot be an empty")
	mockLogger.EXPECT().Error("containerConnectionString cannot be an empty")

	client.Connect()

	require.True(t, mockLogger.ctrl.Satisfied(), "Config Validation Failed")
}

func TestConnect_ProducerError(t *testing.T) {
	ctrl := gomock.NewController(t)

	logs := testutil.StdoutOutputForFunc(func() {
		cfg := getTestConfigs()
		cfg.ConnectionString += ";EntityPath=<entity path>"

		client := New(cfg)

		mockLogger := NewMockLogger(ctrl)

		client.UseLogger(mockLogger)
		client.UseMetrics(NewMockMetrics(ctrl))

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()

		mockLogger.EXPECT().Errorf("error occurred while creating producer client %v", gomock.Any())

		client.Connect()
	})

	require.NotContains(t, logs, "Error")
}

func TestConnect_ContainerError(t *testing.T) {
	ctrl := gomock.NewController(t)

	logs := testutil.StdoutOutputForFunc(func() {
		cfg := getTestConfigs()
		cfg.ContainerConnectionString += "<entity path>"

		client := New(cfg)

		mockLogger := NewMockLogger(ctrl)

		client.UseLogger(mockLogger)
		client.UseMetrics(NewMockMetrics(ctrl))

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()

		mockLogger.EXPECT().Errorf("error occurred while creating container client %v", gomock.Any())

		client.Connect()
	})

	require.NotContains(t, logs, "Error")
}

func TestPublish_ClientNotConnected(t *testing.T) {
	client := New(getTestConfigs())

	err := client.Publish(t.Context(), client.cfg.EventhubName, []byte("my-message"))

	require.ErrorIs(t, err, errClientNotConnected, "Event Hub Publish should fail when producer is not connected")
}

func TestPublish_FailedInvalidTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("Event Hub connection started using connection string")
	mockLogger.EXPECT().Debug("Event Hub producer client setup success")
	mockLogger.EXPECT().Debug("Event Hub container client setup success")
	mockLogger.EXPECT().Debug("Event Hub blobstore client setup success")
	mockLogger.EXPECT().Debug("Event Hub consumer client setup success")
	mockLogger.EXPECT().Debug("Event Hub processor setup success")
	mockLogger.EXPECT().Debug("Event Hub processor running successfully").AnyTimes()
	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug("Event Hub client initialization complete")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.Publish(t.Context(), "random topic", []byte("my-message"))

	require.Equal(t, "topic should be same as Event Hub name", err.Error(), "Event Hub Publish Failed Invalid Topic")

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Publish Failed Invalid Topic")
}

func Test_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("Event Hub connection started using connection string")
	mockLogger.EXPECT().Debug("Event Hub producer client setup success")
	mockLogger.EXPECT().Debug("Event Hub container client setup success")
	mockLogger.EXPECT().Debug("Event Hub blobstore client setup success")
	mockLogger.EXPECT().Debug("Event Hub consumer client setup success")
	mockLogger.EXPECT().Debug("Event Hub processor setup success")
	mockLogger.EXPECT().Debug("Event Hub processor running successfully").AnyTimes()
	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug("Event Hub client initialization complete")
	mockLogger.EXPECT().Error("topic creation is not supported in Event Hub")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.CreateTopic(t.Context(), "random-topic")

	require.NoError(t, err, "Event Hub Topic Creation not allowed failed")

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Topic Creation not allowed failed")
}

func Test_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("Event Hub connection started using connection string")
	mockLogger.EXPECT().Debug("Event Hub producer client setup success")
	mockLogger.EXPECT().Debug("Event Hub container client setup success")
	mockLogger.EXPECT().Debug("Event Hub blobstore client setup success")
	mockLogger.EXPECT().Debug("Event Hub consumer client setup success")
	mockLogger.EXPECT().Debug("Event Hub processor setup success")
	mockLogger.EXPECT().Debug("Event Hub processor running successfully").AnyTimes()
	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug("Event Hub client initialization complete")
	mockLogger.EXPECT().Error("topic deletion is not supported in Event Hub")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.DeleteTopic(t.Context(), "random-topic")

	require.NoError(t, err, "Event Hub Topic Deletion not allowed failed")

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Topic Deletion not allowed failed")
}

func Test_HealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("Event Hub connection started using connection string")
	mockLogger.EXPECT().Debug("Event Hub producer client setup success")
	mockLogger.EXPECT().Debug("Event Hub container client setup success")
	mockLogger.EXPECT().Debug("Event Hub blobstore client setup success")
	mockLogger.EXPECT().Debug("Event Hub consumer client setup success")
	mockLogger.EXPECT().Debug("Event Hub processor setup success")
	mockLogger.EXPECT().Debug("Event Hub processor running successfully").AnyTimes()
	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug("Event Hub client initialization complete")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	// getTestConfigs points at "<your-namespace>.servicebus.windows.net", which is not a legal
	// hostname and so can never resolve -- the probe fails the same way on a runner with or
	// without network access.
	start := time.Now()

	health := client.Health()

	elapsed := time.Since(start)

	// The connectivity probe must stay bounded. Without that bound a health endpoint backed by an
	// unreachable namespace blocks on the SDK's own retry schedule, which is what makes a liveness
	// probe time out instead of answering. The assertion allows 2x eventHubPropsTimeout rather than
	// the timeout itself: the point is that the probe returns on its own deadline instead of the
	// SDK's, and a margin that tight would flake on a loaded runner. Measured, it returns in 1x.
	require.Less(t, elapsed, 2*eventHubPropsTimeout,
		"Health must return within 2x eventHubPropsTimeout (%v), took %v", 2*eventHubPropsTimeout, elapsed)

	require.Equal(t, datasource.StatusDown, health.Status, "Event Hub health should be down when the namespace is unreachable")
	require.Equal(t, "EVENT_HUB", health.Details["backend"])
	require.Equal(t, client.cfg.EventhubName, health.Details["eventHub"])
	require.Contains(t, health.Details, "error", "an unreachable Event Hub should report the probe error")

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Health Check Failed")
}

func getTestConfigs() Config {
	newWebSocketConnFn := func(ctx context.Context, args azeventhubs.WebSocketConnParams) (net.Conn, error) {
		opts := &websocket.DialOptions{
			Subprotocols: []string{"amqp"},
		}

		wssConn, _, err := websocket.Dial(ctx, args.Host, opts)
		if err != nil {
			return nil, err
		}

		return websocket.NetConn(ctx, wssConn, websocket.MessageBinary), nil
	}

	// For more details on the configuration refer :
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/messaging/azeventhubs/consumer_client_test.go
	return Config{
		ConnectionString: "Endpoint=sb://<your-namespace>.servicebus.windows.net/;SharedAccessKeyName=<key-" +
			"name>;SharedAccessKey=<key>",
		ContainerConnectionString: "DefaultEndpointsProtocol=https;AccountName=<storage-account-name>;AccountKey=" +
			"SGVsbG8gV29ybGQ=",
		StorageServiceURL:    "core.windows.net",
		StorageContainerName: "<storage-account-name>",
		EventhubName:         "event-hub-name",
		ConsumerOptions: &azeventhubs.ConsumerClientOptions{
			RetryOptions: azeventhubs.RetryOptions{},
		},
		ProducerOptions: &azeventhubs.ProducerClientOptions{
			NewWebSocketConn: newWebSocketConnFn,
		},
	}
}

func TestGetEventHubName(t *testing.T) {
	expectedName := "test-event-hub"
	client := New(Config{
		EventhubName: expectedName,
	})

	require.Equal(t, expectedName, client.GetEventHubName(),
		"GetEventHubName should return the configured EventhubName")
}

func TestQuery_Failures(t *testing.T) {
	testCases := []struct {
		name          string
		setupClient   func() *Client
		query         string
		expectedError error
	}{
		{
			name: "consumer_not_connected",
			setupClient: func() *Client {
				return New(Config{
					EventhubName: "test-hub",
				})
			},
			query:         "test-hub",
			expectedError: errClientNotConnected,
		},
		{
			name: "empty_topic",
			setupClient: func() *Client {
				client := New(Config{
					EventhubName: "test-hub",
				})
				client.consumer = &azeventhubs.ConsumerClient{}
				return client
			},
			query:         "",
			expectedError: errEmptyTopic,
		},
		{
			name: "topic_mismatch",
			setupClient: func() *Client {
				client := New(Config{
					EventhubName: "test-hub",
				})
				client.consumer = &azeventhubs.ConsumerClient{} // Just needs to be non-nil
				return client
			},
			query:         "different-hub",
			expectedError: ErrTopicMismatch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			client := tc.setupClient()
			mockLogger := NewMockLogger(ctrl)

			client.UseLogger(mockLogger)

			result, err := client.Query(t.Context(), tc.query)

			require.Nil(t, result, "Result should be nil for failure case: %s", tc.name)
			require.Equal(t, tc.expectedError, err, "Error should match expected for case: %s", tc.name)
		})
	}
}

func TestQuery_ContextWithDeadline(t *testing.T) {
	// Test that when context has deadline, we respect it
	ctrl := gomock.NewController(t)

	client := New(Config{
		EventhubName: "test-hub",
	})
	client.consumer = &azeventhubs.ConsumerClient{} // Just needs to be non-nil

	mockLogger := NewMockLogger(ctrl)
	client.UseLogger(mockLogger)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Execute Query (will fail with ErrTopicMismatch before it gets to the deadline handling)
	_, err := client.Query(ctx, "different-hub")

	// Verify it failed for the right reason
	require.Equal(t, ErrTopicMismatch, err)
	require.True(t, mockLogger.ctrl.Satisfied())
}

func Test_ValidConfigs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	client := New(Config{})
	client.UseLogger(mockLogger)

	mockLogger.EXPECT().Error("eventhubName cannot be an empty")
	mockLogger.EXPECT().Error("connectionString cannot be an empty")
	mockLogger.EXPECT().Error("storageServiceURL cannot be an empty")
	mockLogger.EXPECT().Error("storageContainerName cannot be an empty")
	mockLogger.EXPECT().Error("containerConnectionString cannot be an empty")

	valid := client.validConfigs(Config{})

	require.False(t, valid, "validConfigs should return false for invalid configuration")
}

func Test_Health(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	client := New(getTestConfigs())
	client.UseLogger(mockLogger)

	// Without Connect() the consumer is nil, so the health check should short-circuit to down.
	health := client.Health()

	require.Equal(t, datasource.StatusDown, health.Status, "Health should be down when the client is not connected")
	require.Equal(t, "EVENT_HUB", health.Details["backend"])
	require.Equal(t, client.cfg.EventhubName, health.Details["eventHub"])
	require.Equal(t, errClientNotConnected.Error(), health.Details["error"])
	require.NotContains(t, health.Details, "partitionCount", "an unconnected client has no partitions to report")
}

func TestCreateTopic_ForMigrations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	client := New(getTestConfigs())
	client.UseLogger(mockLogger)

	err := client.CreateTopic(t.Context(), "gofr_migrations")

	require.NoError(t, err, "CreateTopic should not return an error for 'gofr_migrations'")
}

func Test_GetEventHubName(t *testing.T) {
	expectedName := "test-event-hub"
	client := New(Config{
		EventhubName: expectedName,
	})

	actualName := client.GetEventHubName()

	require.Equal(t, expectedName, actualName, "GetEventHubName should return the configured EventhubName")
}

func TestConnect_ConsumerGroupDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := getTestConfigs()
	cfg.ConsumerGroup = ""

	client := New(cfg)
	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debugf("Using default consumer group: %s", azeventhubs.DefaultConsumerGroup)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(NewMockMetrics(ctrl))

	client.Connect()

	require.Equal(t, azeventhubs.DefaultConsumerGroup, client.cfg.ConsumerGroup,
		"Client should automatically switch to $Default consumer group when config is empty")
}

func TestConnect_ConsumerGroupProvided(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := getTestConfigs()
	expectedGroup := "my-custom-group"
	cfg.ConsumerGroup = expectedGroup

	client := New(cfg)
	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debugf("Using provided consumer group: %s", expectedGroup)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(NewMockMetrics(ctrl))

	client.Connect()

	require.Equal(t, expectedGroup, client.cfg.ConsumerGroup, "Client should respect the provided consumer group")
}

// mockConsumerClient is a hand-written stand-in for *azeventhubs.ConsumerClient, in the same
// shape as the SQS client's mockSQSClient: the call under test is scripted through a func field
// and the others answer with a fixed value. Close reports success; NewPartitionClient reports an
// error, for the reason given on it below.
type mockConsumerClient struct {
	getPropsFunc func(ctx context.Context,
		options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error)
}

func (m *mockConsumerClient) GetEventHubProperties(ctx context.Context,
	options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	if m.getPropsFunc != nil {
		return m.getPropsFunc(ctx, options)
	}

	return azeventhubs.EventHubProperties{}, nil
}

// NewPartitionClient returns an error rather than (nil, nil). The concrete return type can be
// constructed -- &azeventhubs.PartitionClient{} compiles -- but not into anything usable: every
// field is unexported, so ReceiveEvents nil-derefs on it. And handing back a nil client would
// nil-deref on the deferred Close in the first test that reached tryReadFromPartition.
func (*mockConsumerClient) NewPartitionClient(string,
	*azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	return nil, errPartitionClientUnavailable
}

func (*mockConsumerClient) Close(context.Context) error { return nil }

// newHealthTestClient returns a client that looks connected to Health without a live namespace.
func newHealthTestClient(t *testing.T, consumer consumerClient) *Client {
	t.Helper()

	client := New(getTestConfigs())
	client.UseLogger(NewMockLogger(gomock.NewController(t)))
	client.consumer = consumer

	return client
}

// Test_Health_Connected covers the only branch that reports the backend as usable. Nothing in
// the down paths can reach it, so without a stubbed consumer StatusUp and partitionCount are
// never executed by the suite at all.
func Test_Health_Connected(t *testing.T) {
	client := newHealthTestClient(t, &mockConsumerClient{
		getPropsFunc: func(context.Context,
			*azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
			return azeventhubs.EventHubProperties{
				Name:         "event-hub-name",
				PartitionIDs: []string{"0", "1", "2", "3"},
			}, nil
		},
	})

	health := client.Health()

	require.Equal(t, datasource.StatusUp, health.Status, "a reachable Event Hub must report up")
	require.Equal(t, "EVENT_HUB", health.Details["backend"])
	require.Equal(t, client.cfg.EventhubName, health.Details["eventHub"])
	require.Equal(t, 4, health.Details["partitionCount"], "partitionCount must be the number of partitions reported")
	require.NotContains(t, health.Details, "error", "a healthy Event Hub must not report an error")
}

// Test_Health_ProbeError pins the message an operator actually reads when the probe fails. The
// live-namespace test below can only ever produce "context deadline exceeded", so the error is
// passed through verbatim only here.
func Test_Health_ProbeError(t *testing.T) {
	client := newHealthTestClient(t, &mockConsumerClient{
		getPropsFunc: func(context.Context,
			*azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
			return azeventhubs.EventHubProperties{}, errHealthProbe
		},
	})

	health := client.Health()

	require.Equal(t, datasource.StatusDown, health.Status, "a failing probe must report down")
	require.Equal(t, errHealthProbe.Error(), health.Details["error"], "the probe error must reach the caller verbatim")
	require.NotContains(t, health.Details, "partitionCount", "a failed probe has no partition count to report")
}

// Test_Health_NotConnectedDoesNotProbe pins the short-circuit itself: an unconnected client must
// report down without dialing, so a probe on a dead pod costs nothing and cannot block.
func Test_Health_NotConnectedDoesNotProbe(t *testing.T) {
	// A nil consumer, not a scripted mock. There is no seam that can observe a call which must not
	// happen -- a mock passed here is discarded by the nil assignment, so its t.Error could never
	// fire. If the short-circuit is removed, Health calls GetEventHubProperties on a nil interface
	// and this test panics; that is the failure signal.
	client := newHealthTestClient(t, nil)

	health := client.Health()

	require.Equal(t, datasource.StatusDown, health.Status)
	require.Equal(t, errClientNotConnected.Error(), health.Details["error"])
}

// Test_Health_BoundsAProbeThatIgnoresContext is the case the Azure SDK actually creates.
// GetEventHubProperties passes context.Background() to the AMQP round-trip, so handing our
// deadline to the SDK does not bound it; only selecting on the deadline ourselves does. The fake
// here blocks without ever reading ctx, which is what the SDK does once a management link exists
// and the broker stops answering.
//
// Without probeWithin's select this test does not fail -- it hangs until the go test timeout.
func Test_Health_BoundsAProbeThatIgnoresContext(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	client := newHealthTestClient(t, &mockConsumerClient{
		getPropsFunc: func(context.Context,
			*azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
			<-release

			return azeventhubs.EventHubProperties{}, nil
		},
	})

	start := time.Now()
	health := client.Health()
	elapsed := time.Since(start)

	require.Less(t, elapsed, 2*eventHubPropsTimeout,
		"Health must return on its own deadline even when the probe ignores the context, took %v", elapsed)
	require.Equal(t, datasource.StatusDown, health.Status, "an unanswered probe must report down")
	require.Equal(t, context.DeadlineExceeded.Error(), health.Details["error"],
		"the caller must see the deadline, not a nil error")
	require.NotContains(t, health.Details, "partitionCount", "an unanswered probe has no partition count")
}
