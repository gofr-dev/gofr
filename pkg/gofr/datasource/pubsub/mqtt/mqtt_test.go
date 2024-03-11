package mqtt

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/testutil"
)

var errTest = errors.New("test error")

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
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		client = New(&conf, mockLogger, nil)
	})

	assert.NotNil(t, client.Client)
	assert.Contains(t, out, "cannot connect to MQTT")
}

// TestMQTT_EmptyConfigs test the scenario where configs are not provided and
// client tries to connect to the public broker.
func TestMQTT_EmptyConfigs(t *testing.T) {
	var client *MQTT

	out := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.DEBUGLOG)
		client = New(&Config{}, mockLogger, nil)
	})

	assert.NotNil(t, client)
	assert.Contains(t, out, "connected to MQTT")
	assert.Contains(t, out, "Port : 1883")
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
	options := getMQTTClientOptions(&conf, testutil.NewMockLogger(testutil.ERRORLOG))

	assert.Contains(t, options.ClientID, conf.ClientID)
	assert.ElementsMatch(t, options.Servers, []*url.URL{expectedURL})
	assert.Equal(t, conf.Username, options.Username)
	assert.Equal(t, conf.Password, options.Password)
	assert.Equal(t, conf.Order, options.Order)
}

func TestMQTT_Ping(t *testing.T) {
	m := New(&Config{}, testutil.NewMockLogger(testutil.FATALLOG), nil)

	// Success Case
	err := m.Ping()
	assert.Nil(t, err)

	// Disconnect the client
	m.Disconnect(1)

	// Failure Case
	err = m.Ping()
	assert.NotNil(t, err)
	assert.Equal(t, errClientNotConnected, err)
}

func TestMQTT_Disconnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
	client := New(&Config{}, mockLogger, mockMetrics)

	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test")

	// Disconnect the broker and then try to publish
	client.Disconnect(1)

	err := client.Publish(ctx, "test", []byte("hello"))
	assert.NotNil(t, err)
	assert.Equal(t, "not Connected", err.Error())
}

func TestMQTT_PublishSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test/topic")
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", "test/topic")

	m := New(&Config{}, testutil.NewMockLogger(testutil.FATALLOG), mockMetrics)

	err := m.Publish(ctx, "test/topic", []byte(`hello world`))

	assert.Nil(t, err)
}

func TestMQTT_PublishFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	mockMetrics := NewMockMetrics(ctrl)
	out := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)

		// case where the client has been disconnected, resulting in a Publishing failure
		mockMetrics.EXPECT().
			IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test/topic")
		m := New(&Config{}, mockLogger, mockMetrics)

		// Disconnect the client
		m.Client.Disconnect(1)
		err := m.Publish(ctx, "test/topic", []byte(`hello world`))

		assert.NotNil(t, err)
	})

	assert.Contains(t, out, "error while publishing")
}

func TestMQTT_SubscribeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)

	// expect the publishing metric calls
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", "test/topic")
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", "test/topic")

	// expect the subcscibers metric calls
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", "test/topic")
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", "test/topic")

	m := New(&Config{QoS: 0}, mockLogger, mockMetrics)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		msg, err := m.Subscribe(ctx, "test/topic")

		assert.NotNil(t, msg)
		assert.Equal(t, "test/topic", msg.Topic)

		assert.Nil(t, err)
	}()

	_ = m.Publish(ctx, "test/topic", []byte("hello world"))

	wg.Wait()
}

func TestMQTT_SubscribeFailure(t *testing.T) {
	// Subscribing on a disconncted client
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	mockMetrics := NewMockMetrics(ctrl)
	mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)

	// expect the subcscibers metric calls
	mockMetrics.EXPECT().
		IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", "test/topic")

	m := New(&Config{QoS: 0}, mockLogger, mockMetrics)

	// Disconnect the client
	m.Client.Disconnect(1)
	msg, err := m.Subscribe(ctx, "test/topic")

	assert.NotNil(t, err)
	assert.Nil(t, msg)
}

func TestMQTT_SubscribeWithFunc(t *testing.T) {
	subcriptionFunc := func(msg *pubsub.Message) error {
		assert.NotNil(t, msg)
		assert.Equal(t, "test/topic", msg.Topic)

		return nil
	}

	subcriptionFuncErr := func(msg *pubsub.Message) error {
		return errTest
	}

	m := New(&Config{}, testutil.NewMockLogger(testutil.ERRORLOG), nil)

	// Success case
	err := m.SubscribeWithFunction("test/topic", subcriptionFunc)
	assert.Nil(t, err)

	// Error case where error is returned from subscription function
	err = m.SubscribeWithFunction("test/topic", subcriptionFuncErr)
	assert.Nil(t, err)

	// Unsubscribe from the topic
	_ = m.Unsubscribe("test/topic")

	// Error case where the client cannot connect
	m.Disconnect(1)
	err = m.SubscribeWithFunction("test/topic", subcriptionFunc)
	assert.NotNil(t, err)
}

func TestMQTT_Unsubscribe(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		m := New(&Config{}, mockLogger, nil)

		// Success case
		err := m.Unsubscribe("test/topic")
		assert.Nil(t, err)

		// Failure case
		m.Client.Disconnect(1)
		err = m.Unsubscribe("test/topic")
		assert.NotNil(t, err)
	})

	assert.Contains(t, out, "error while unsubscribing from topic test/topic")
}

func TestMQTT_CreateTopic(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		m := New(&Config{}, mockLogger, nil)

		// Success case
		err := m.CreateTopic(context.TODO(), "test/topic")
		assert.Nil(t, err)

		// Failure case
		m.Client.Disconnect(1)
		err = m.CreateTopic(context.TODO(), "test/topic")
		assert.NotNil(t, err)
	})

	assert.Contains(t, out, "unable to create topic - test/topic")
}

func TestMQTT_Health(t *testing.T) {
	// The Client is not configured(nil)
	out := testutil.StderrOutputForFunc(func() {
		m := &MQTT{config: &Config{}, logger: testutil.NewMockLogger(testutil.ERRORLOG)}
		res := m.Health()
		assert.Equal(t, datasource.Health{
			Status:  "DOWN",
			Details: map[string]interface{}{"backend": "MQTT", "host": ""},
		}, res)
	})

	assert.Contains(t, out, "datasource not initialized")

	// The client ping fails
	out = testutil.StderrOutputForFunc(func() {
		m := New(&Config{}, testutil.NewMockLogger(testutil.ERRORLOG), nil)

		m.Disconnect(1)

		res := m.Health()
		assert.Equal(t, datasource.Health{
			Status:  "DOWN",
			Details: map[string]interface{}{"backend": "MQTT", "host": publicBroker},
		}, res)
	})

	assert.Contains(t, out, "health check failed")

	// Success Case - Status UP
	_ = testutil.StderrOutputForFunc(func() {
		m := New(&Config{}, testutil.NewMockLogger(testutil.ERRORLOG), nil)
		res := m.Health()
		assert.Equal(t, datasource.Health{
			Status:  "UP",
			Details: map[string]interface{}{"backend": "MQTT", "host": publicBroker},
		}, res)
	})
}

// Non-functional test case; for coverage purposes.
func TestMQTT_DeleteTopic(_ *testing.T) {
	m := &MQTT{}
	_ = m.DeleteTopic(context.TODO(), "")
}
