package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource"
)

func testHealthCheck(t *testing.T, client *testRedisClient) {
	t.Helper()

	h := client.PubSub.Health()
	assert.Equal(t, "UP", h.Status)
	assert.Equal(t, "streams", h.Details["mode"]) // Default mode is now streams
}

func TestPubSub_HealthDown(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	mock.ExpectPing().SetErr(errMockPing)

	h := client.PubSub.Health()
	assert.Equal(t, datasource.StatusDown, h.Status)
	assert.Equal(t, "REDIS", h.Details["backend"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HealthUp(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	mock.ExpectPing().SetVal("PONG")

	h := client.PubSub.Health()
	assert.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, "REDIS", h.Details["backend"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HealthDetails(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_PORT":        "6380",
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	mock.ExpectPing().SetVal("PONG")

	h := client.PubSub.Health()
	assert.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, "REDIS", h.Details["backend"])
	assert.Equal(t, "localhost:6380", h.Details["addr"])
	assert.Equal(t, "pubsub", h.Details["mode"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HealthDefaultMode(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_HOST": "localhost",
		"REDIS_PORT": "6379",
	})
	defer client.Close()

	mock.ExpectPing().SetVal("PONG")

	h := client.PubSub.Health()
	require.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, "streams", h.Details["mode"], "should default to streams when not specified")

	assert.NoError(t, mock.ExpectationsWereMet())
}
