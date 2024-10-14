package container

import (
	"context"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	ws "gofr.dev/pkg/gofr/websocket"
)

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))

	container := NewContainer(cfg)

	assert.NotNil(t, container.Logger, "TEST, Failed.\nlogger initialisation")
}

func Test_newContainerDBInitializationFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "invalid")
	t.Setenv("DB_DIALECT", "mysql")
	t.Setenv("DB_HOST", "invalid")

	cfg := config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))

	container := NewContainer(cfg)

	db := container.SQL.(*gofrSql.DB)
	redis := container.Redis.(*gofrRedis.Redis)

	// container is a pointer, and we need to see if db are not initialized, comparing the container object
	// will not suffice the purpose of this test
	require.Error(t, db.DB.Ping(), "TEST, Failed.\ninvalid db connections")
	assert.NotNil(t, redis.Client, "TEST, Failed.\ninvalid redis connections")
}

func Test_newContainerPubSubInitializationFail(t *testing.T) {
	testCases := []struct {
		desc    string
		configs map[string]string
	}{
		{
			desc: "Google PubSub fail",
			configs: map[string]string{
				"PUBSUB_BACKEND": "GOOGLE",
			},
		},
	}

	for _, tc := range testCases {
		c := NewContainer(config.NewMockConfig(tc.configs))

		assert.Nil(t, c.PubSub)
	}
}

func TestContainer_MQTTInitialization_Default(t *testing.T) {
	configs := map[string]string{
		"PUBSUB_BACKEND": "MQTT",
	}

	c := NewContainer(config.NewMockConfig(configs))

	assert.NotNil(t, c.PubSub)
	m, ok := c.PubSub.(*mqtt.MQTT)
	assert.True(t, ok)
	assert.NotNil(t, m.Client)
}

func TestContainer_GetHTTPService(t *testing.T) {
	svc := service.NewHTTPService("", nil, nil)

	c := &Container{Services: map[string]service.HTTP{
		"test-service": svc,
	}}

	testCases := []struct {
		desc       string
		servicName string
		expected   service.HTTP
	}{
		{
			desc:       "success get",
			servicName: "test-service",
			expected:   svc,
		},
		{
			desc:       "failed get",
			servicName: "invalid-service",
			expected:   nil,
		},
	}

	for _, tc := range testCases {
		out := c.GetHTTPService(tc.servicName)

		assert.Equal(t, tc.expected, out)
	}
}

func TestContainer_GetAppName(t *testing.T) {
	c := &Container{appName: "test-app"}

	out := c.GetAppName()

	assert.Equal(t, "test-app", out)
}

func TestContainer_GetAppVersion(t *testing.T) {
	c := &Container{appVersion: "v0.1.0"}

	out := c.GetAppVersion()

	assert.Equal(t, "v0.1.0", out)
}

func TestContainer_GetPublisher(t *testing.T) {
	publisher := &MockPubSub{}

	c := &Container{PubSub: publisher}

	out := c.GetPublisher()

	assert.Equal(t, publisher, out)
}

func TestContainer_GetSubscriber(t *testing.T) {
	subscriber := &MockPubSub{}

	c := &Container{PubSub: subscriber}

	out := c.GetSubscriber()

	assert.Equal(t, subscriber, out)
}

func TestContainer_newContainerWithNilConfig(t *testing.T) {
	container := NewContainer(nil)

	failureMsg := "TestContainer_newContainerWithNilConfig Failed!"

	assert.Nil(t, container.Redis, "%s", failureMsg)
	assert.Nil(t, container.SQL, "%s", failureMsg)
	assert.Nil(t, container.Services, "%s", failureMsg)
	assert.Nil(t, container.PubSub, "%s", failureMsg)
	assert.Nil(t, container.Logger, "%s", failureMsg)
}

func TestContainer_Close(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	mockDB, sqlMock, _ := gofrSql.NewSQLMocks(t)
	mockRedis := NewMockRedis(controller)
	mockPubSub := &MockPubSub{}

	mockRedis.EXPECT().Close().Return(nil)
	sqlMock.ExpectClose()

	c := NewContainer(config.NewMockConfig(nil))
	c.SQL = &sqlMockDB{mockDB, &expectedQuery{}, logging.NewLogger(logging.DEBUG)}
	c.Redis = mockRedis
	c.PubSub = mockPubSub

	assert.NotNil(t, c.PubSub)

	err := c.Close()
	require.NoError(t, err)
}

func Test_GetConnectionFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *ws.Connection
	}{
		{
			name:     "no connection in context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name:     "connection in context",
			ctx:      context.WithValue(context.Background(), ws.WSConnectionKey, &ws.Connection{Conn: &websocket.Conn{}}),
			expected: &ws.Connection{Conn: &websocket.Conn{}},
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), ws.WSConnectionKey, "wrong-type"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := (&Container{}).GetConnectionFromContext(tt.ctx)

			assert.Equal(t, tt.expected, conn)
		})
	}
}
