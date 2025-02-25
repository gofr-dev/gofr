package eventhub

import (
	"context"
	"net"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/testutil"
	"nhooyr.io/websocket"
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

	mockMetrics.EXPECT().IncrementCounter(context.Background(), "app_pubsub_publish_total_count", "topic", client.cfg.EventhubName)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.Publish(context.Background(), client.cfg.EventhubName, []byte("my-message"))

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

	err := client.Publish(context.Background(), "random topic", []byte("my-message"))

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
	mockLogger.EXPECT().Error("topic deletion is not supported in Event Hub")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.DeleteTopic(context.Background(), "random-topic")

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
	mockLogger.EXPECT().Error("topic creation is not supported in Event Hub")

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.CreateTopic(context.Background(), "random-topic")

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
