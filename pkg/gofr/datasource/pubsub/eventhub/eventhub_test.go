package eventhub

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"nhooyr.io/websocket"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/testutil"
)

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

		mockLogger.EXPECT().Errorf("error occurred while creating container client %v", gomock.Any())

		client.Connect()
	})

	require.NotContains(t, logs, "Error")
}

func TestPublish_FailedBatchCreation(t *testing.T) {
	// TODO: This test is skipped due to long runtime and occasional panic, causing pipeline failures.
	// It needs modification in the future.
	t.Skip("disabled on 2024-12-11, TODO: cause of occasional panic in this test needs to be addressed.")

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

	mockMetrics.EXPECT().IncrementCounter(t.Context(), "app_pubsub_publish_total_count", "topic", client.cfg.EventhubName)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.Publish(t.Context(), client.cfg.EventhubName, []byte("my-message"))

	require.ErrorContains(t, err, "failed to WebSocket dial: failed to send handshake request: ",
		"Eventhub Publish Failed Batch Creation")

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Publish Failed Batch Creation")
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
	mockLogger.EXPECT().Error("health-check not implemented for Event Hub")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	_ = client.Health()

	require.True(t, mockLogger.ctrl.Satisfied(), "Event Hub Topic Deletion not allowed failed")
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

	mockLogger.EXPECT().Error("health-check not implemented for Event Hub")

	health := client.Health()

	require.Equal(t, datasource.Health{}, health, "Health should return an empty datasource.Health struct")
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
