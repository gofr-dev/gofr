package redis

import (
	"crypto/tls"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelglobal "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_NewClient_HostNameMissing(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "REDIS_HOST empty", env: map[string]string{"REDIS_HOST": ""}},
		{name: "invalid REDIS_URL", env: map[string]string{"REDIS_URL": "unix:///var/run/redis.sock", "REDIS_HOST": "localhost"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := logging.NewMockLogger(logging.ERROR)
			mockMetrics := NewMockMetrics(ctrl)
			mockConfig := config.NewMockConfig(tc.env)

			client := NewClient(mockConfig, mockLogger, mockMetrics)
			assert.Nil(t, client, "Test_NewClient_HostNameMissing Failed! Expected redis client to be nil")
		})
	}
}

func TestNewClient_ConnectFailureLog(t *testing.T) {
	port := strconv.Itoa(testutil.GetFreePort(t))

	tests := []struct {
		name    string
		env     map[string]string
		connect func(config.Config, *MockMetrics) io.Closer
		want    string
	}{
		{
			name:    "client with discrete variables",
			env:     map[string]string{"REDIS_HOST": "127.0.0.1", "REDIS_PORT": port, "REDIS_DB": "3"},
			connect: newTestClient,
			want:    "could not connect to redis at '127.0.0.1:" + port + "' on database 3",
		},
		{
			name:    "client with an IPv6 URL",
			env:     map[string]string{"REDIS_URL": "redis://[::1]:" + port + "/2"},
			connect: newTestClient,
			want:    "could not connect to redis at '[::1]:" + port + "' on database 2",
		},
		{
			name:    "pubsub with an IPv6 URL",
			env:     map[string]string{"PUBSUB_BACKEND": "REDIS", "REDIS_URL": "redis://[::1]:" + port + "/2", "REDIS_PUBSUB_DB": "4"},
			connect: newTestPubSub,
			want:    "could not connect to redis at '[::1]:" + port + "' on database 4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockMetrics := NewMockMetrics(ctrl)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			var closer io.Closer

			stderr := testutil.StderrOutputForFunc(func() {
				closer = tc.connect(config.NewMockConfig(tc.env), mockMetrics)
			})

			require.NotNil(t, closer)

			t.Cleanup(func() { _ = closer.Close() })

			assert.Contains(t, stderr, tc.want)
		})
	}
}

func newTestClient(c config.Config, metrics *MockMetrics) io.Closer {
	return NewClient(c, logging.NewMockLogger(logging.ERROR), metrics)
}

func newTestPubSub(c config.Config, metrics *MockMetrics) io.Closer {
	return NewPubSub(c, logging.NewMockLogger(logging.ERROR), metrics)
}

func Test_NewClient_InvalidPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockMetrics := NewMockMetrics(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "&&^%%^&*"})

	// The go-redis library may send multiple commands during initialization (hello, client, ping, etc.)
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", gomock.Any(),
	).AnyTimes()

	client := NewClient(mockConfig, mockLogger, mockMetrics)
	assert.NotNil(t, client.Client, "Test_NewClient_InvalidPort Failed! Expected redis client not to be nil")
}

func TestRedis_QueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any()).AnyTimes()

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
			"REDIS_DB":   "1",
		}), mockLogger, mockMetric)

		require.NoError(t, err)

		result, err := client.Set(t.Context(), "key", "value", 1*time.Minute).Result()

		require.NoError(t, err)
		assert.Equal(t, "OK", result)
	})

	// Assertions
	assert.Contains(t, result, "set")
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "value")
	assert.Contains(t, result, "ex 60")
}

func TestRedis_PipelineQueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any()).AnyTimes()

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetric)

		require.NoError(t, err)

		// Pipeline execution
		pipe := client.Pipeline()
		setCmd := pipe.Set(t.Context(), "key1", "value1", 1*time.Minute)
		getCmd := pipe.Get(t.Context(), "key1")

		// Pipeline Exec should return a non-nil error
		_, err = pipe.Exec(t.Context())
		require.NoError(t, err)

		// Retrieve results
		setResult, err := setCmd.Result()
		require.NoError(t, err)
		assert.Equal(t, "OK", setResult)

		getResult, err := getCmd.Result()
		require.NoError(t, err)
		assert.Equal(t, "value1", getResult)
	})

	// Assertions
	// All Redis commands are now logged, including pipeline operations
	assert.Contains(t, result, "connected to redis")
}

func TestRedis_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	// Mock metrics setup
	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname",
		gomock.Any(), "type", gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := NewClient(config.NewMockConfig(map[string]string{
		"REDIS_HOST": s.Host(),
		"REDIS_PORT": s.Port(),
	}), mockLogger, mockMetric)

	err = client.Close()

	require.NoError(t, err)
}

func Test_TLSConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
	})

	conf := getRedisConfig(mockConfig, mockLogger)
	assert.NotNil(t, conf.TLS, "Expected TLS config to be set")
	assert.EqualValues(t, tls.VersionTLS12, conf.TLS.MinVersion, "Expected TLS 1.2")
}

func Test_TLSConfigWithDummyPEM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPEM := getMockPEM()
	mockKey := getMockKey()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CA_CERT": mockPEM,
		"REDIS_TLS_CERT":    mockPEM,
		"REDIS_TLS_KEY":     mockKey,
	})

	conf := getRedisConfig(mockConfig, mockLogger)
	assert.NotNil(t, conf.TLS, "Expected TLS config to be set")
	assert.EqualValues(t, tls.VersionTLS12, conf.TLS.MinVersion, "Expected TLS 1.2")
}

func getMockPEM() string {
	const mockPEM = `-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnzQw\n-----END CERTIFICATE-----`

	return mockPEM
}

func getMockKey() string {
	const mockKey = `-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAnzQw\n-----END RSA PRIVATE KEY-----`

	return mockKey
}

func TestSetPubSubDB(t *testing.T) {
	tests := []struct {
		desc    string
		conf    map[string]string
		options *redis.Options
		expDB   int
	}{
		{desc: "default when not configured", conf: map[string]string{}, options: &redis.Options{}, expDB: defaultPubSubDB},
		{desc: "configured value", conf: map[string]string{"REDIS_PUBSUB_DB": "3"}, options: &redis.Options{}, expDB: 3},
		{desc: "non numeric value falls back to default", conf: map[string]string{"REDIS_PUBSUB_DB": "abc"},
			options: &redis.Options{DB: 1}, expDB: defaultPubSubDB},
		{desc: "negative value falls back to default", conf: map[string]string{"REDIS_PUBSUB_DB": "-1"},
			options: &redis.Options{DB: 1}, expDB: defaultPubSubDB},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := &Config{DB: 1, Options: tc.options}

			setPubSubDB(config.NewMockConfig(tc.conf), cfg)

			assert.Equal(t, tc.expDB, cfg.DB)
			assert.Equal(t, tc.expDB, cfg.Options.DB)
		})
	}
}

func TestSetPubSubDB_NilOptions(t *testing.T) {
	tests := []struct {
		desc  string
		conf  map[string]string
		expDB int
	}{
		{desc: "valid value", conf: map[string]string{"REDIS_PUBSUB_DB": "4"}, expDB: 4},
		{desc: "invalid value", conf: map[string]string{"REDIS_PUBSUB_DB": "x"}, expDB: defaultPubSubDB},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := &Config{}

			setPubSubDB(config.NewMockConfig(tc.conf), cfg)

			assert.Equal(t, tc.expDB, cfg.DB)
			assert.Nil(t, cfg.Options)
		})
	}
}

func TestNewPubSub_HostNameMissing(t *testing.T) {
	ctrl := gomock.NewController(t)

	ps := NewPubSub(config.NewMockConfig(map[string]string{"REDIS_HOST": ""}),
		logging.NewMockLogger(logging.ERROR), NewMockMetrics(ctrl))

	assert.Nil(t, ps)
}

func TestRedis_Close_NilClient(t *testing.T) {
	r := &Redis{stopSignal: make(chan struct{})}

	require.NoError(t, r.Close())
	require.NoError(t, r.Close(), "closing twice must not panic on the stop signal")
}

func TestRetryConnect(t *testing.T) {
	testCases := []struct {
		desc          string
		hideArgs      bool
		wantStatement bool
	}{
		{desc: "statement recorded by default", hideArgs: false, wantStatement: true},
		{desc: "statement dropped when hidden", hideArgs: true, wantStatement: false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			prevTP := otelglobal.GetTracerProvider()

			otelglobal.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
			t.Cleanup(func() { otelglobal.SetTracerProvider(prevTP) })

			// Redis is down when the client is created and comes up while retryConnect waits.
			s, err := miniredis.Run()
			require.NoError(t, err)

			defer s.Close()

			// miniredis cannot report its address while stopped.
			addr, port := s.Addr(), s.Port()

			s.Close()

			rc := redis.NewClient(&redis.Options{Addr: addr})
			defer rc.Close()

			done := make(chan struct{})

			go func() {
				retryConnect(rc, &Config{hideArgs: tc.hideArgs}, logging.NewMockLogger(logging.ERROR),
					10*time.Millisecond, make(chan struct{}))
				close(done)
			}()

			require.NoError(t, s.Restart())

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("retryConnect did not connect after Redis came up")
			}

			require.NoError(t, rc.Set(t.Context(), "secret-key", "secret-value", 0).Err())

			setSpan := findSpan(exporter.GetSpans(), "set", port)
			require.NotNil(t, setSpan, "no set span was exported for this Redis")

			assert.Equal(t, tc.wantStatement, hasAttribute(setSpan.Attributes, "db.statement"))
		})
	}
}

// findSpan returns the last span with the given name sent to the Redis on port, or nil.
// Matching the port keeps out spans from clients of other tests that use the global provider.
func findSpan(spans tracetest.SpanStubs, name, port string) *tracetest.SpanStub {
	var found *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name != name {
			continue
		}

		for _, a := range spans[i].Attributes {
			if a.Key == "server.port" && strconv.FormatInt(a.Value.AsInt64(), 10) == port {
				found = &spans[i]
			}
		}
	}

	return found
}
