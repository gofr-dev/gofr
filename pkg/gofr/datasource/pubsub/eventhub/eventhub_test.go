package eventhub

import (
	"context"
	"net"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"

	"nhooyr.io/websocket"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/testutil"
)

func TestConnect(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug("azure eventhub connection started using connection string")
	mockLogger.EXPECT().Debug("azure eventhub producer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub container client setup success")
	mockLogger.EXPECT().Debug("azure eventhub blobstore client setup success")
	mockLogger.EXPECT().Debug("azure eventhub consumer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor running successfully").AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(NewMockMetrics(ctrl))

	client.Connect()

	require.True(t, mockLogger.ctrl.Satisfied(), "Eventhub Connection Failed")
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
		cfg.ConnectionString = cfg.ConnectionString + ";EntityPath=<entity path>"

		client := New(cfg)

		mockLogger := NewMockLogger(ctrl)

		client.UseLogger(mockLogger)
		client.UseMetrics(NewMockMetrics(ctrl))

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		mockLogger.EXPECT().Error("error occurred while creating producer client connection string " +
			"contains an EntityPath. eventHub must be an empty string")

		client.Connect()
	})

	require.NotContains(t, logs, "Error")
}

func TestConnect_ContainerError(t *testing.T) {
	ctrl := gomock.NewController(t)

	logs := testutil.StdoutOutputForFunc(func() {
		cfg := getTestConfigs()
		cfg.ContainerConnectionString = cfg.ContainerConnectionString + "<entity path>"

		client := New(cfg)

		mockLogger := NewMockLogger(ctrl)

		client.UseLogger(mockLogger)
		client.UseMetrics(NewMockMetrics(ctrl))

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		mockLogger.EXPECT().Error("error occurred while creating container client decode account key:" +
			" illegal base64 data at input byte 16")

		client.Connect()
	})

	require.NotContains(t, logs, "Error")
}

func TestPublish_FailedBatchCreation(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("azure eventhub connection started using connection string")
	mockLogger.EXPECT().Debug("azure eventhub producer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub container client setup success")
	mockLogger.EXPECT().Debug("azure eventhub blobstore client setup success")
	mockLogger.EXPECT().Debug("azure eventhub consumer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor running successfully").AnyTimes()

	mockMetrics.EXPECT().IncrementCounter(context.Background(), "app_pubsub_publish_total_count", "topic", client.cfg.EventhubName)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.Publish(context.Background(), client.cfg.EventhubName, []byte("my-message"))

	require.NotNil(t, err)

	require.True(t, mockLogger.ctrl.Satisfied(), "Eventhub Publish Failed Batch Creation")
}

func TestPublish_FailedInvalidTopic(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := New(getTestConfigs())

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug("azure eventhub connection started using connection string")
	mockLogger.EXPECT().Debug("azure eventhub producer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub container client setup success")
	mockLogger.EXPECT().Debug("azure eventhub blobstore client setup success")
	mockLogger.EXPECT().Debug("azure eventhub consumer client setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor setup success")
	mockLogger.EXPECT().Debug("azure eventhub processor running successfully").AnyTimes()

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.Connect()

	err := client.Publish(context.Background(), "random topic", []byte("my-message"))

	require.Equal(t, "topic should be same as eventhub name", err.Error(), "Eventhub Publish Failed Invalid Topic")

	require.True(t, mockLogger.ctrl.Satisfied(), "Eventhub Publish Failed Invalid Topic")
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

	// For more details on the configuration refer https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/messaging/azeventhubs/consumer_client_test.go
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
