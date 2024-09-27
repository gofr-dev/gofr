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

// TODO write tests for error cases

func TestConnect(t *testing.T) {
	ctrl := gomock.NewController(t)

	logs := testutil.StderrOutputForFunc(func() {
		client := New(getTestConfigs())

		mockLogger := NewMockLogger(ctrl)

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		client.UseLogger(mockLogger)
		client.UseMetrics(NewMockMetrics(ctrl))

		client.Connect()
	})

	// TODO check if mocks are satisfied
	require.NotContains(t, logs, "Error")
}

func TestConfigValidation(t *testing.T) {
	ctrl := gomock.NewController(t)

	logger := NewMockLogger(ctrl)

	testutil.StderrOutputForFunc(func() {
		client := New(Config{})

		client.UseLogger(logger)

		logger.EXPECT().Error("EventhubName cannot be an empty")
		logger.EXPECT().Error("ConnectionString cannot be an empty")
		logger.EXPECT().Error("StorageServiceURL cannot be an empty")
		logger.EXPECT().Error("StorageContainerName cannot be an empty")
		logger.EXPECT().Error("ContainerConnectionString cannot be an empty")

		client.Connect()
	})

	require.True(t, logger.ctrl.Satisfied(), "Config Validation Failed")
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
