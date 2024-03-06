package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	assert.NotNil(t, container.Logger, "TEST, Failed.\nlogger initialisation")
}

func Test_newContainerDBIntializationFail(t *testing.T) {
	t.Setenv("REDIS_HOST", "invalid")
	t.Setenv("DB_DIALECT", "mysql")
	t.Setenv("DB_HOST", "invalid")

	cfg := config.NewEnvFile("")

	container := NewContainer(cfg)

	// container is a pointer and we need to see if db are not initialized, comparing the container object
	// will not suffice the purpose of this test
	assert.Nil(t, container.SQL.DB, "TEST, Failed.\ninvalid db connections")
	assert.Nil(t, container.Redis.Client, "TEST, Failed.\ninvalid redis connections")
}

func Test_newContainerPubSubIntializationFail(t *testing.T) {
	testCases := []struct {
		desc    string
		configs map[string]string
	}{
		{
			desc: "Kafka failure",
			configs: map[string]string{
				"PUBSUB_BACKEND": "KAFKA",
				"PUBSUB_BROKER":  "invalid",
			},
		},
		{
			desc: "Google PubSub fail",
			configs: map[string]string{
				"PUBSUB_BACKEND": "GOOGLE",
			},
		},
	}

	for _, tc := range testCases {
		c := NewContainer(testutil.NewMockConfig(tc.configs))

		assert.Nil(t, c.PubSub)
	}
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
	publisher := &mockPubSub{}

	c := &Container{PubSub: publisher}

	out := c.GetPublisher()

	assert.Equal(t, publisher, out)
}

func TestContainer_GetSubscriber(t *testing.T) {
	subscriber := &mockPubSub{}

	c := &Container{PubSub: subscriber}

	out := c.GetSubscriber()

	assert.Equal(t, subscriber, out)
}

func TestContainer_NewEmptyContainer(t *testing.T) {
	container := NewEmptyContainer()

	assert.Nil(t, container.Redis, "TestContainer_NewEmptyContainer Failed!")
	assert.Nil(t, container.SQL, "TestContainer_NewEmptyContainer Failed")
	assert.Nil(t, container.Services, "TestContainer_NewEmptyContainer Failed")
	assert.Nil(t, container.PubSub, "TestContainer_NewEmptyContainer Failed")
	assert.Nil(t, container.Logger, "TestContainer_NewEmptyContainer Failed")
}

type mockPubSub struct {
}

func (m *mockPubSub) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (m *mockPubSub) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (m *mockPubSub) Health() datasource.Health {
	return datasource.Health{}
}

func (m *mockPubSub) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *mockPubSub) Subscribe(_ context.Context, _ string) (*pubsub.Message, error) {
	return nil, nil
}
