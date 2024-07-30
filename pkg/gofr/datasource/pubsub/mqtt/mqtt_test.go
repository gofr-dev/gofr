package mqtt

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errToken = errors.New("connection error with MQTT")
	errTest  = errors.New("test error")
)

var (
	//nolint:gochecknoglobals //used for testing purposes only
	mockConfigs = &Config{
		Protocol:         "tcp",
		Hostname:         "localhost",
		Port:             1883,
		Username:         "admin",
		Password:         "admin",
		ClientID:         "abc2222",
		QoS:              1,
		Order:            false,
		RetrieveRetained: false,
		KeepAlive:        0,
	}
	//nolint:gochecknoglobals //used for testing purposes only
	msg = []byte(`hello world`)
)

func TestMQTT_New(t *testing.T) {
	var client *MQTT

	conf := Config{
		Protocol:         "tcp",
		Hostname:         "localhost",
		Port:             1883,
		QoS:              0,
		Order:            false,
		RetrieveRetained: false,
	}

	out := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.ERROR)
		client = New(&conf, mockLogger, nil)
	})

	assert.NotNil(t, client.Client)
	assert.Contains(t, out, "could not connect to MQTT")
}

// TestMQTT_EmptyConfigs test the scenario where configs are not provided and
// a client tries to connect to the public broker.
func TestMQTT_EmptyConfigs(t *testing.T) {
	var client *MQTT

	out := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client = New(&Config{}, mockLogger, nil)
	})

	assert.NotNil(t, client)
	assert.Contains(t, out, "connected to MQTT")
}

func TestMQTT_getMQTTClientOptions(t *testing.T) {
	conf := Config{
		Protocol: "tcp",
		Hostname: "localhost",
		Port:     1883,
		QoS:      0,
		Username: "user",
		Password: "pass",
		ClientID: "test",
		Order:    false,
	}

	expectedURL, _ := url.Parse("tcp://localhost:1883")
	options := getMQTTClientOptions(&conf)

	assert.Contains(t, options.ClientID, conf.ClientID)
	assert.ElementsMatch(t, options.Servers, []*url.URL{expectedURL})
	assert.Equal(t, conf.Username, options.Username)
	assert.Equal(t, conf.Password, options.Password)
	assert.Equal(t, conf.Order, options.Order)
}

func TestMQTT_Ping(t *testing.T) {
	ctrl, mq, mockClient, _, _ := getMockMQTT(t, nil)
	defer ctrl.Finish()

	mockClient.EXPECT().IsConnected().Return(true)
	// Success Case
	err := mq.Ping()
	require.NoError(t, err)

	mockClient.EXPECT().Disconnect(uint(1))

	// Disconnect the client
	_ = mq.Disconnect(1)

	mockClient.EXPECT().IsConnected().Return(false)
	// Failure Case
	err = mq.Ping()
	require.Error(t, err)
	assert.Equal(t, errClientNotConnected, err)
}

func TestMQTT_Disconnect(t *testing.T) {
	ctrl, client, mockClient, mockMetrics, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	ctx := context.Background()

	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test")

	mockClient.EXPECT().Disconnect(uint(1))

	mockClient.EXPECT().Publish("test", mockConfigs.QoS, mockConfigs.RetrieveRetained, msg).Return(mockToken)

	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(errToken).Times(3)

	// Disconnect the broker and then try to publish
	_ = client.Disconnect(1)

	err := client.Publish(ctx, "test", msg)
	require.Error(t, err)
	require.ErrorIs(t, err, errToken)
}

func TestMQTT_DisconnectWithSubscriptions(t *testing.T) {
	subs := make(map[string]subscription)
	subs["test/topic"] = subscription{
		msgs:    make(chan *pubsub.Message),
		handler: func(_ mqtt.Client, _ mqtt.Message) {},
	}

	ctrl, client, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	client.subscriptions = subs

	mockClient.EXPECT().Disconnect(uint(1))
	mockClient.EXPECT().Unsubscribe("test/topic").Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(nil)

	_ = client.Disconnect(1)

	// we assert that on unsubscribing the subscription gets deleted
	_, ok := client.subscriptions["test/topic"]
	assert.False(t, ok)
}

func TestMQTT_PublishSuccess(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		ctrl, client, mockClient, mockMetrics, mockToken := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		ctx := context.Background()

		mockMetrics.EXPECT().
			IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test/topic")
		mockMetrics.EXPECT().
			IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", "test/topic")

		mockClient.EXPECT().Publish("test/topic", mockConfigs.QoS, mockConfigs.RetrieveRetained, msg).
			Return(mockToken)

		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(nil)

		err := client.Publish(ctx, "test/topic", msg)

		require.NoError(t, err)
	})

	assert.Contains(t, out, "PUB")
	assert.Contains(t, out, "MQTT")
	assert.Contains(t, out, "hello world")
	assert.Contains(t, out, "test/topic")
}

func TestMQTT_PublishFailure(t *testing.T) {
	ctrl, client, mockClient, mockMetrics, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	ctx := context.Background()
	// case where the client has been disconnected, resulting in a Publishing failure
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test/topic")

	mockClient.EXPECT().Disconnect(uint(1))
	// Disconnect the client
	_ = client.Disconnect(1)

	mockClient.EXPECT().Publish("test/topic", mockConfigs.QoS, mockConfigs.RetrieveRetained, msg).Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(errToken).Times(3)

	err := client.Publish(ctx, "test/topic", []byte(`hello world`))

	require.Error(t, err)
	require.ErrorIs(t, err, errToken)
}

func TestMQTT_SubscribeSuccess(t *testing.T) {
	ctrl, client, mockClient, mockMetrics, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	ctx := context.Background()

	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", "test/topic")

	mockClient.EXPECT().Subscribe("test/topic", mockConfigs.QoS, gomock.Any()).Return(mockToken)

	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(nil)

	go func() {
		client.subscriptions["test/topic"].msgs <- &pubsub.Message{
			Topic:     "test/topic",
			Value:     msg,
			MetaData:  nil,
			Committer: &message{msg: mockMessage{}},
		}
	}()

	m, err := client.Subscribe(ctx, "test/topic")

	assert.Equal(t, msg, m.Value)
	require.NoError(t, err)
}

func TestMQTT_SubscribeFailure(t *testing.T) {
	ctrl, client, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	ctx := context.Background()

	mockClient.EXPECT().Subscribe("test/topic", mockConfigs.QoS, gomock.Any()).Return(mockToken)

	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(errToken).Times(3)

	m, err := client.Subscribe(ctx, "test/topic")

	assert.Nil(t, m)
	require.Error(t, err)
	require.ErrorIs(t, err, errToken)
}

func TestMQTT_SubscribeWithFunc(t *testing.T) {
	ctrl, client, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	subscriptionFunc := func(msg *pubsub.Message) error {
		assert.Equal(t, "test/topic", msg.Topic)

		return nil
	}

	subscriptionFuncErr := func(*pubsub.Message) error {
		return errTest
	}

	// Success case
	mockClient.EXPECT().Subscribe("test/topic", mockConfigs.QoS, gomock.Any()).Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(nil)

	err := client.SubscribeWithFunction("test/topic", subscriptionFunc)
	require.NoError(t, err)

	// Error case where error is returned from subscription function
	mockClient.EXPECT().Subscribe("test/topic", mockConfigs.QoS, gomock.Any()).Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(nil)

	err = client.SubscribeWithFunction("test/topic", subscriptionFuncErr)
	require.NoError(t, err)

	// Unsubscribe from the topic
	mockClient.EXPECT().Unsubscribe("test/topic").Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(nil)

	_ = client.Unsubscribe("test/topic")

	// Error case where the client cannot connect
	mockClient.EXPECT().Disconnect(uint(1))
	_ = client.Disconnect(1)

	mockClient.EXPECT().Subscribe("test/topic", mockConfigs.QoS, gomock.Any()).Return(mockToken)
	mockToken.EXPECT().Wait().Return(true)
	mockToken.EXPECT().Error().Return(errToken).Times(2)

	err = client.SubscribeWithFunction("test/topic", subscriptionFunc)
	require.Error(t, err)
	require.ErrorIs(t, err, errToken)
}

func Test_getHandler(t *testing.T) {
	subscriptionFunc := func(msg *pubsub.Message) error {
		assert.Equal(t, []byte("hello from sub func"), msg.Value)
		assert.Equal(t, map[string]string{"qos": string(byte(1)), "retained": "false", "messageID": "123"}, msg.MetaData)
		assert.Equal(t, "test/topic", msg.Topic)

		return nil
	}

	h := getHandler(subscriptionFunc)

	h(nil, mockMessage{
		duplicate: false,
		qos:       1,
		retained:  false,
		topic:     "test/topic",
		messageID: 123,
		pyload:    "hello from sub func",
	})
}

func TestMQTT_Unsubscribe(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		ctrl, client, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		// Success case
		mockClient.EXPECT().Unsubscribe("test/topic").Return(mockToken)
		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(nil)

		err := client.Unsubscribe("test/topic")
		require.NoError(t, err)

		// Failure case
		mockClient.EXPECT().Disconnect(uint(1))
		_ = client.Disconnect(1)

		mockClient.EXPECT().Unsubscribe("test/topic").Return(mockToken)
		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(errToken).Times(3)

		err = client.Unsubscribe("test/topic")
		require.Error(t, err)
	})

	assert.Contains(t, out, "error while unsubscribing from topic 'test/topic'")
}

func TestMQTT_CreateTopic(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		ctrl, client, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		// Success case
		mockClient.EXPECT().
			Publish("test/topic", mockConfigs.QoS, mockConfigs.RetrieveRetained, []byte("topic creation")).
			Return(mockToken)
		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(nil)

		err := client.CreateTopic(context.Background(), "test/topic")
		require.NoError(t, err)

		// Failure case
		mockClient.EXPECT().Disconnect(uint(1))
		client.Client.Disconnect(1)

		mockClient.EXPECT().
			Publish("test/topic", mockConfigs.QoS, mockConfigs.RetrieveRetained, []byte("topic creation")).
			Return(mockToken)
		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(errToken).Times(3)

		err = client.CreateTopic(context.Background(), "test/topic")
		require.Error(t, err)
	})

	assert.Contains(t, out, "unable to create topic 'test/topic'")
}

func TestMQTT_Health(t *testing.T) {
	// The Client is not configured(nil)
	out := testutil.StderrOutputForFunc(func() {
		m := &MQTT{config: &Config{}, logger: logging.NewMockLogger(logging.ERROR)}
		res := m.Health()
		assert.Equal(t, datasource.Health{
			Status:  "DOWN",
			Details: map[string]interface{}{"backend": "MQTT", "host": ""},
		}, res)
	})

	assert.Contains(t, out, "datasource not initialized")

	// The client ping fails
	out = testutil.StderrOutputForFunc(func() {
		ctrl, client, mockClient, _, _ := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		mockClient.EXPECT().IsConnected().Return(false)

		res := client.Health()
		assert.Equal(t, datasource.Health{
			Status:  "DOWN",
			Details: map[string]interface{}{"backend": "MQTT", "host": "localhost"},
		}, res)
	})

	assert.Contains(t, out, "health check failed")

	// Success Case - Status UP
	_ = testutil.StderrOutputForFunc(func() {
		ctrl, client, mockClient, _, _ := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		mockClient.EXPECT().IsConnected().Return(true)

		res := client.Health()
		assert.Equal(t, datasource.Health{
			Status:  "UP",
			Details: map[string]interface{}{"backend": "MQTT", "host": "localhost"},
		}, res)
	})
}

func TestMQTT_DeleteTopic(t *testing.T) {
	m := &MQTT{}

	err := m.DeleteTopic(context.Background(), "test/topic")
	require.NoError(t, err)
}

func TestReconnectingHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Infof("reconnecting to MQTT at '%v:%v' with clientID '%v'", "any", 1234, "gopher")

	handler := createReconnectingHandler(mockLogger, &Config{
		Hostname: "any",
		Port:     1234,
		ClientID: "gopher",
	})
	assert.NotNil(t, handler)

	handler(NewMockClient(ctrl), &mqtt.ClientOptions{})
}

func TestConnectionLostHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Errorf("mqtt connection lost, error: %v", gomock.Any()).
		DoAndReturn(func(_ string, args ...interface{}) {
			assert.Len(t, args, 1)
			require.Error(t, mqtt.ErrNotConnected, args[0])
		})

	connectionLostHandler := createConnectionLostHandler(mockLogger)
	connectionLostHandler(NewMockClient(ctrl), mqtt.ErrNotConnected)
}

func TestReconnectHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	qos := byte(1)

	clientMock := NewMockClient(ctrl)
	clientMock.EXPECT().Subscribe("topic1", qos, gomock.Any()).Return(nil)

	msgsChan := make(chan *pubsub.Message)

	defer close(msgsChan)

	subs := map[string]subscription{
		"topic1": {
			msgs:    msgsChan,
			handler: func(_ mqtt.Client, _ mqtt.Message) {},
		},
	}

	reconnectHandler := createReconnectHandler(&sync.RWMutex{}, &Config{
		Hostname: "any",
		Port:     1234,
		ClientID: "gopher",
		QoS:      qos,
	}, subs)

	assert.NotNil(t, reconnectHandler)

	reconnectHandler(clientMock)
}

func TestMQTT_createMqttHandler(t *testing.T) {
	var (
		out  string
		msgs = make(chan *pubsub.Message)
		wg   = sync.WaitGroup{}
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		out = testutil.StdoutOutputForFunc(func() {
			ctrl, client, _, mockMetrics, _ := getMockMQTT(t, mockConfigs)
			defer ctrl.Finish()

			mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", "test/topic")

			handler := client.createMqttHandler(context.Background(), "test/topic", msgs)

			handler(nil, mockMessage{false, 1, false, "test/topic", 123, "hello world"})
		})
	}()

	m := <-msgs
	close(msgs)

	assert.NotNil(t, m)
	assert.Equal(t, m.Value, msg)

	// wait for the goroutine test to finish writing log to out
	wg.Wait()
	assert.Contains(t, out, "SUB")
	assert.Contains(t, out, "hello world")
	assert.Contains(t, out, "test/topic")
	assert.Contains(t, out, "MQTT")
}

func getMockMQTT(t *testing.T, conf *Config) (*gomock.Controller, *MQTT, *MockClient, *MockMetrics, *MockToken) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockClient := NewMockClient(ctrl)
	mockToken := NewMockToken(ctrl)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	mockMetrics := NewMockMetrics(ctrl)

	mq := &MQTT{mockClient, mockLogger, mockMetrics, conf, make(map[string]subscription), &sync.RWMutex{}}

	return ctrl, mq, mockClient, mockMetrics, mockToken
}

func TestMQTT_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctrl, client, mockMQTT, _, _ := getMockMQTT(t, mockConfigs)
	defer ctrl.Finish()

	mockMQTT.EXPECT().Disconnect(uint(0 * time.Millisecond))

	// Close the client
	err := client.Close()

	require.NoError(t, err)
}
