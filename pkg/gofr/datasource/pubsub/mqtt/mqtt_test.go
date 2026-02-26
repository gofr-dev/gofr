package mqtt

import (
	"context"
	"errors"
	"net/url"
	"strconv"
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
		client = New(&Config{Username: "gofr-mqtt-test"}, mockLogger, nil)
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

	ctx := t.Context()

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

		ctx := t.Context()

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

	ctx := t.Context()
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

	ctx := t.Context()

	mockMetrics.EXPECT().
		IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", "test/topic")
	mockClient.EXPECT().IsConnected().Return(true)
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

	ctx := t.Context()

	mockClient.EXPECT().IsConnected().Return(true)
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

		err := client.CreateTopic(t.Context(), "test/topic")
		require.NoError(t, err)

		// Failure case
		mockClient.EXPECT().Disconnect(uint(1))
		client.Client.Disconnect(1)

		mockClient.EXPECT().
			Publish("test/topic", mockConfigs.QoS, mockConfigs.RetrieveRetained, []byte("topic creation")).
			Return(mockToken)
		mockToken.EXPECT().Wait().Return(true)
		mockToken.EXPECT().Error().Return(errToken).Times(3)

		err = client.CreateTopic(t.Context(), "test/topic")
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
			Details: map[string]any{"backend": "MQTT", "host": ""},
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
			Details: map[string]any{"backend": "MQTT", "host": "localhost"},
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
			Details: map[string]any{"backend": "MQTT", "host": "localhost"},
		}, res)
	})
}

func TestMQTT_DeleteTopic(t *testing.T) {
	m := &MQTT{}

	err := m.DeleteTopic(t.Context(), "test/topic")
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
		DoAndReturn(func(_ string, args ...any) {
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
	tokenMock := NewMockToken(ctrl)

	clientMock.EXPECT().Subscribe("topic1", qos, gomock.Any()).Return(tokenMock)

	tokenMock.EXPECT().Wait().Return(true)
	tokenMock.EXPECT().Error().Return(nil)

	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Debugf("resubscribed to topic %s successfully", "topic1")

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
	}, subs, mockLogger)

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

			handler := client.createMqttHandler(t.Context(), "test/topic", msgs)

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

func Test_parseQueryArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []any
		expectedTimeout time.Duration
		expectedLimit   int
	}{
		{"no args", []any{}, defaultQueryCollectTimeout, defaultQueryMessageLimit},
		{"only timeout arg", []any{10 * time.Second}, 10 * time.Second, defaultQueryMessageLimit},
		{"timeout and limit args", []any{15 * time.Second, 5}, 15 * time.Second, 5},
		{"invalid timeout type, valid limit", []any{"not a duration", 5}, defaultQueryCollectTimeout, 5},
		{"valid timeout, invalid limit type", []any{10 * time.Second, "not an int"}, 10 * time.Second, defaultQueryMessageLimit},
		{"only limit arg (nil for timeout)", []any{nil, 7}, defaultQueryCollectTimeout, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout, limit := parseQueryArgs(tt.args...)
			assert.Equal(t, tt.expectedTimeout, timeout, "Timeout mismatch")
			assert.Equal(t, tt.expectedLimit, limit, "Limit mismatch")
		})
	}
}

func TestMQTT_createQueryMessageHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mq := &MQTT{logger: mockLogger}

	msgChan := make(chan *pubsub.Message, 1) // Buffer size of 1
	topic := "test/handler/topic"

	handler := mq.createQueryMessageHandler(t.Context(), msgChan, topic)

	mockMsg1 := &mockMessage{topic: topic, pyload: "message 1", messageID: 123, qos: 1, retained: false}
	mockMsg2 := &mockMessage{topic: topic, pyload: "message 2 (dropped)", messageID: 124, qos: 1, retained: false}

	// Send first message
	handler(nil, mockMsg1)

	// Expect a log when the second message is dropped due to buffer overflow
	mockLogger.EXPECT().Debugf("Query: msgChan full for topic %s, message dropped during collection", topic).Times(1)

	// Send second message, which should be dropped
	handler(nil, mockMsg2)

	// Assert the received message (single assertion block)
	receivedMsg := <-msgChan
	assert.Equal(t, mockMsg1.Topic(), receivedMsg.Topic)
	assert.Equal(t, mockMsg1.Payload(), receivedMsg.Value)

	meta := receivedMsg.MetaData.(map[string]string)
	assert.Equal(t, map[string]string{
		"messageID": strconv.Itoa(int(mockMsg1.MessageID())),
		"qos":       string(mockMsg1.Qos()),
		"retained":  strconv.FormatBool(mockMsg1.Retained()),
	}, meta, "Metadata mismatch")

	// Ensure the channel is empty
	assert.Empty(t, msgChan, "Channel should be empty after reading the first message")
}

func TestMQTT_subscribeToTopicForQuery_SuccessAndErrors(t *testing.T) {
	topic := "test/subquery"
	timeout := 50 * time.Millisecond
	dummyHandler := func(_ mqtt.Client, _ mqtt.Message) {}

	testCases := []struct {
		name        string
		waitSuccess bool
		tokenErr    error
		expectedErr error
	}{
		{"success", true, nil, nil},
		{"timeout without token error", false, nil, errSubscriptionTimeout},
		{"timeout with token error", false, errToken, errSubscriptionFailed},
		{"completed but token error", true, errToken, errSubscriptionFailed},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, mq, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
			defer ctrl.Finish()

			mockClient.EXPECT().Subscribe(topic, mockConfigs.QoS, gomock.Any()).Return(mockToken)
			mockToken.EXPECT().WaitTimeout(timeout).Return(tc.waitSuccess)
			mockToken.EXPECT().Error().Return(tc.tokenErr)

			err := mq.subscribeToTopicForQuery(t.Context(), topic, timeout, dummyHandler)
			assert.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

func TestMQTT_subscribeToTopicForQuery_ContextError(t *testing.T) {
	topicName := "test/subquery"
	subscribeTimeout := 50 * time.Millisecond // Short timeout for tests

	var dummyHandler mqtt.MessageHandler = func(_ mqtt.Client, _ mqtt.Message) {}

	t.Run("error_context_canceled_during_wait", func(t *testing.T) {
		ctrl, mq, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		cancelledCtx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel context immediately

		mockClient.EXPECT().Subscribe(topicName, mockConfigs.QoS, gomock.Any()).Return(mockToken)
		mockToken.EXPECT().WaitTimeout(subscribeTimeout).Return(false) // Assume WaitTimeout returns false due to context

		err := mq.subscribeToTopicForQuery(cancelledCtx, topicName, subscribeTimeout, dummyHandler)

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled) // Check that the specific context error is wrapped
		assert.Contains(t, err.Error(), "context error during MQTT subscription to '"+topicName+"': context canceled")
	})

	t.Run("error_context_deadline_exceeded_during_wait", func(t *testing.T) {
		ctrl, mq, mockClient, _, mockToken := getMockMQTT(t, mockConfigs)
		defer ctrl.Finish()

		deadlineCtx, cancel := context.WithTimeout(t.Context(), 1*time.Nanosecond) // Ensure deadline exceeded
		defer cancel()

		time.Sleep(5 * time.Millisecond) // Give time for deadline to pass

		mockClient.EXPECT().Subscribe(topicName, mockConfigs.QoS, gomock.Any()).Return(mockToken)
		mockToken.EXPECT().WaitTimeout(subscribeTimeout).Return(false)

		err := mq.subscribeToTopicForQuery(deadlineCtx, topicName, subscribeTimeout, dummyHandler)

		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "context error during MQTT subscription to '"+topicName+"': context deadline exceeded")
	})
}

func TestMQTT_Query_SuccessCases(t *testing.T) {
	topic := "test/query/success"

	testCases := []struct {
		name         string
		messageLimit int
		timeout      time.Duration
		messages     []string
		expectedData string
	}{
		{"one_message_default_args", 1, 150 * time.Millisecond, []string{"hello query one"}, "hello query one"},
		{"multiple_messages_limit_reached", 2, 150 * time.Millisecond, []string{"msgA", "msgB", "msgC"}, "msgA\nmsgB"},
		{"timeout_reached_before_limit", 5, 30 * time.Millisecond, []string{"only"}, "only"},
		{"no_messages_collected", 1, 30 * time.Millisecond, []string{}, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, mq, mockClient, _, mockSubToken := getMockMQTT(t, mockConfigs)
			defer ctrl.Finish()

			mockClient.EXPECT().IsConnected().Return(true)

			var capturedHandler mqtt.MessageHandler

			mockClient.EXPECT().Subscribe(topic, mockConfigs.QoS, gomock.Any()).
				DoAndReturn(func(_ string, _ byte, h mqtt.MessageHandler) mqtt.Token {
					capturedHandler = h
					return mockSubToken
				})

			mockSubToken.EXPECT().WaitTimeout(tc.timeout).Return(true)
			mockSubToken.EXPECT().Error().Return(nil)

			mockUnsubToken := NewMockToken(ctrl)
			mockClient.EXPECT().Unsubscribe(topic).Return(mockUnsubToken)
			mockUnsubToken.EXPECT().WaitTimeout(unsubscribeOpTimeout).Return(true)

			// Simulate message publishing
			go func() {
				time.Sleep(10 * time.Millisecond)

				for _, msg := range tc.messages {
					capturedHandler(nil, &mockMessage{topic: topic, pyload: msg, qos: int(mockConfigs.QoS)})
				}
			}()

			result, err := mq.Query(t.Context(), topic, tc.timeout, tc.messageLimit)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedData, string(result))
		})
	}
}
