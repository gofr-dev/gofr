package redis

import (
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
	assert.Equal(t, "localhost:6380", h.Details["host"])
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

// newMiniredisClient returns a go-redis client and matching Config pointing at a miniredis server.
// When connected is false the server is stopped so that every command fails.
func newMiniredisClient(t *testing.T, connected bool) (*redis.Client, *Config) {
	t.Helper()

	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)

	addr, host := s.Addr(), s.Host()
	port, err := strconv.Atoi(s.Port())
	require.NoError(t, err)

	if !connected {
		s.Close()
	}

	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1})

	t.Cleanup(func() { _ = client.Close() })

	return client, &Config{HostName: host, Port: port}
}

func TestRedis_HealthCheck(t *testing.T) {
	tests := []struct {
		desc        string
		connected   bool
		client      func(c *redis.Client) *redis.Client
		expStatus   string
		expHasStats bool
		expHasError bool
	}{
		{
			desc:        "client not initialized",
			connected:   true,
			client:      func(*redis.Client) *redis.Client { return nil },
			expStatus:   datasource.StatusDown,
			expHasError: true,
		},
		{
			desc:        "server unreachable",
			client:      func(c *redis.Client) *redis.Client { return c },
			expStatus:   datasource.StatusDown,
			expHasError: true,
		},
		{
			desc:        "server reachable",
			connected:   true,
			client:      func(c *redis.Client) *redis.Client { return c },
			expStatus:   datasource.StatusUp,
			expHasStats: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, cfg := newMiniredisClient(t, tc.connected)

			r := &Redis{Client: tc.client(client), config: cfg}

			h := r.HealthCheck()

			_, hasStats := h.Details["stats"]
			_, hasError := h.Details["error"]

			assert.Equal(t, tc.expStatus, h.Status)
			assert.Equal(t, client.Options().Addr, h.Details["host"])
			assert.Equal(t, tc.expHasStats, hasStats)
			assert.Equal(t, tc.expHasError, hasError)
		})
	}
}

func TestPubSub_Health_DefaultModeFallback(t *testing.T) {
	tests := []struct {
		desc      string
		connected bool
		expStatus string
	}{
		{desc: "connected with empty mode", connected: true, expStatus: datasource.StatusUp},
		{desc: "disconnected with empty mode", connected: false, expStatus: datasource.StatusDown},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ps := newZeroConfigPubSub(t, tc.connected)

			h := ps.Health()

			assert.Equal(t, tc.expStatus, h.Status)
			assert.Equal(t, modeStreams, h.Details["mode"])
			assert.Equal(t, redisBackend, h.Details["backend"])
		})
	}
}
