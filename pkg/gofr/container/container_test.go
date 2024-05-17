package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
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
	assert.Error(t, db.DB.Ping(), "TEST, Failed.\ninvalid db connections")
	assert.Nil(t, redis.Client, "TEST, Failed.\ninvalid redis connections")
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

func TestLoggerMasking(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		maskingEnabled string
		maskingFields  string
		expectedFields []string
	}{
		{
			name:           "Masking enabled with multiple fields",
			maskingFields:  "password,email,creditCard",
			expectedFields: []string{"password", "email", "creditCard"},
		},
		{
			name:           "Masking enabled with single field",
			maskingFields:  "password",
			expectedFields: []string{"password"},
		},
		{
			name:           "Masking disabled",
			maskingFields:  "",
			expectedFields: []string{},
		},
		{
			name:           "Masking enabled with empty fields",
			maskingFields:  "password,,email,  ,creditCard",
			expectedFields: []string{"password", "email", "creditCard"},
		},
	}

	// Iterate over test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock configuration
			mockConfig := config.NewMockConfig(map[string]string{
				"LOGGER_MASKING_FIELDS": tc.maskingFields,
				"LOG_LEVEL":             "INFO",
			})

			// Create a new container using the mock configuration
			c := NewContainer(mockConfig)

			// Get the logger from the container
			logger := c.Logger

			// Get the actual masking fields from the logger
			actualFields := logger.GetMaskingFilters()

			// Compare the lengths of the actual fields and expected fields
			if len(actualFields) != len(tc.expectedFields) {
				t.Errorf("Expected masking fields length: %d, but got: %d", len(tc.expectedFields), len(actualFields))
			}

			// Compare the actual fields with the expected fields
			for i := range actualFields {
				if actualFields[i] != tc.expectedFields[i] {
					t.Errorf("Expected masking field: %s, but got: %s", tc.expectedFields[i], actualFields[i])
				}
			}
		})
	}
}
