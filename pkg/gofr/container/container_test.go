package container

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/service"
	ws "gofr.dev/pkg/gofr/websocket"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_newContainerSuccessWithLogger(t *testing.T) {
	cfg := config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))

	container := NewContainer(cfg)

	assert.NotNil(t, container.Logger, "TEST, Failed.\nlogger initialization")
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
	require.Error(t, db.DB.PingContext(t.Context()), "TEST, Failed.\ninvalid db connections")
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
		setup    func(c *Container)
		expected *ws.Connection
	}{
		{
			name:     "no connection in context",
			ctx:      t.Context(),
			setup:    func(*Container) {},
			expected: nil,
		},
		{
			name: "connection in context",
			ctx:  context.WithValue(t.Context(), ws.WSConnectionKey, "test-conn-id"),
			setup: func(c *Container) {
				c.WSManager = ws.New()
				c.WSManager.AddWebsocketConnection("test-conn-id", &ws.Connection{Conn: &websocket.Conn{}})
			},
			expected: &ws.Connection{Conn: &websocket.Conn{}},
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(t.Context(), ws.WSConnectionKey, 12345),
			setup:    func(*Container) {},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{}
			tt.setup(container)

			conn := container.GetConnectionFromContext(tt.ctx)

			assert.Equal(t, tt.expected, conn)
		})
	}
}

func TestContainer_CreateSetsAppNameAndVersion(t *testing.T) {
	// Test case: Explicit values are provided
	t.Run("explicit config values", func(t *testing.T) {
		cfg := config.NewMockConfig(map[string]string{
			"APP_NAME":    "test-app",
			"APP_VERSION": "v1.0.0",
		})

		c := &Container{}
		c.Create(cfg)

		assert.Equal(t, "test-app", c.GetAppName())
		assert.Equal(t, "v1.0.0", c.GetAppVersion())
	})

	// Test case: Empty config should use default values
	t.Run("empty config uses defaults", func(t *testing.T) {
		cfg := config.NewMockConfig(map[string]string{}) // No values provided

		c := &Container{}
		c.Create(cfg)

		assert.Equal(t, "gofr-app", c.GetAppName())
		assert.Equal(t, "dev", c.GetAppVersion())
	})
}

func TestRedisPubSubEffectiveMode(t *testing.T) {
	tests := []struct {
		desc     string
		mode     string
		expected string
	}{
		{desc: "explicit pubsub", mode: "pubsub", expected: redisPubSubModePubSub},
		{desc: "explicit streams", mode: "streams", expected: redisPubSubModeStreams},
		{desc: "empty defaults to streams", mode: "", expected: redisPubSubModeStreams},
		{desc: "invalid falls back to streams", mode: "invalid", expected: redisPubSubModeStreams},
	}

	for _, tc := range tests {
		conf := config.NewMockConfig(map[string]string{"REDIS_PUBSUB_MODE": tc.mode})
		require.Equal(t, tc.expected, effectiveRedisPubSubMode(conf), tc.desc)
	}
}

func TestWarnRedisPubSubSharedDB_NoWarnWhenRedisNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := &Container{Logger: NewMockLogger(ctrl)}

	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
		"REDIS_DB":          "0",
	}))
}

func TestWarnRedisPubSubSharedDB_NoWarnWhenModeIsPubSub(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := &Container{
		Logger: NewMockLogger(ctrl),
		Redis:  NewMockRedis(ctrl), // non-nil
	}

	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
		"REDIS_DB":          "0",
	}))
}

func TestWarnRedisPubSubSharedDB_WarnsWhenPubSubDBUnset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := &Container{
		Logger: NewMockLogger(ctrl),
		Redis:  NewMockRedis(ctrl), // non-nil
	}

	// No warning expected when REDIS_PUBSUB_DB is unset (defaults to DB 15, which is safe)
	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
		"REDIS_DB":          "0",
	}))
}

func TestWarnRedisPubSubSharedDB_WarnsWhenPubSubDBInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := &Container{
		Logger: NewMockLogger(ctrl),
		Redis:  NewMockRedis(ctrl), // non-nil
	}

	// No warning expected when REDIS_PUBSUB_DB is invalid (will use default DB 15, which is safe)
	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
		"REDIS_DB":          "0",
		"REDIS_PUBSUB_DB":   "not-a-number",
	}))
}

func TestWarnRedisPubSubSharedDB_WarnsWhenPubSubDBEqualsRedisDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var warns []string

	logger := NewMockLogger(ctrl)
	// Warnf is called with format string + 2 integer arguments (pubsubDB, redisDB)
	logger.EXPECT().Warnf(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(format string, args ...any) {
		warns = append(warns, fmt.Sprintf(format, args...))
	}).Times(1)

	c := &Container{
		Logger: logger,
		Redis:  NewMockRedis(ctrl), // non-nil
	}

	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
		"REDIS_DB":          "0",
		"REDIS_PUBSUB_DB":   "0",
	}))

	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "migrations may fail")
}

func TestWarnRedisPubSubSharedDB_NoWarnWhenPubSubDBDiffers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := &Container{
		Logger: NewMockLogger(ctrl),
		Redis:  NewMockRedis(ctrl), // non-nil
	}

	c.warnIfRedisPubSubSharesRedisDB(config.NewMockConfig(map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
		"REDIS_DB":          "0",
		"REDIS_PUBSUB_DB":   "1",
	}))
}

func TestCreatePubSub_DispatchBranches(t *testing.T) {
	t.Run("kafka branch with empty broker does nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		c := &Container{Logger: NewMockLogger(ctrl)}
		c.createPubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "KAFKA"}))
		require.Nil(t, c.PubSub)
	})

	t.Run("google branch with missing configs returns nil client", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var errs []string

		logger := NewMockLogger(ctrl)
		// google.New uses Debugf with varying arg counts; allow both shapes.
		logger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debugf(gomock.Any()).AnyTimes()
		logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

		logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Do(func(format string, args ...any) {
			errs = append(errs, fmt.Sprintf(format, args...))
		}).AnyTimes()

		c := &Container{Logger: logger}
		c.createPubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "GOOGLE"}))
		require.Nil(t, c.PubSub)
		require.NotEmpty(t, errs)
	})

	t.Run("redis branch with empty host returns nil client", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		c := &Container{Logger: NewMockLogger(ctrl)}
		c.createPubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "REDIS"}))
		require.Nil(t, c.PubSub)
	})
}

func TestWebsocketManagerHelpers(t *testing.T) {
	m := ws.New()

	c := &Container{
		WSManager: m,
	}

	connID := "svc-1"
	conn := &ws.Connection{}

	c.AddConnection(connID, conn)

	got := c.GetWSConnectionByServiceName(connID)
	require.Equal(t, conn, got)

	c.RemoveConnection(connID)
	require.Nil(t, c.GetWSConnectionByServiceName(connID))
}

func TestContainer_registerFrameworkMetrics_RegistersExpectedMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	c := &Container{
		Logger:         NewMockLogger(ctrl),
		metricsManager: mockMetrics,
	}

	gauges := []string{
		"app_info",
		"app_go_routines",
		"app_sys_memory_alloc",
		"app_sys_total_alloc",
		"app_go_numGC",
		"app_go_sys",
		"app_sql_open_connections",
		"app_sql_inUse_connections",
	}
	for _, gauge := range gauges {
		mockMetrics.EXPECT().NewGauge(gauge, gomock.Any()).Times(1)
	}

	mockMetrics.EXPECT().NewGauge("app_http_circuit_breaker_state", gomock.Any()).Times(1)

	counters := []string{
		"app_pubsub_publish_total_count",
		"app_pubsub_publish_success_count",
		"app_pubsub_subscribe_total_count",
		"app_pubsub_subscribe_success_count",
		"app_http_retry_count",
	}
	for _, counter := range counters {
		mockMetrics.EXPECT().NewCounter(counter, gomock.Any()).Times(1)
	}

	httpBuckets := []float64{.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30}
	dsBuckets := getDefaultDatasourceBuckets()

	histograms := []struct {
		name    string
		buckets []float64
	}{
		{name: "app_http_response", buckets: httpBuckets},
		{name: "app_http_service_response", buckets: httpBuckets},
		{name: "app_redis_stats", buckets: dsBuckets},
		{name: "app_sql_stats", buckets: dsBuckets},
	}

	for _, tc := range histograms {
		bucketMatchers := make([]any, 0, len(tc.buckets))
		for range tc.buckets {
			bucketMatchers = append(bucketMatchers, gomock.Any())
		}

		mockMetrics.EXPECT().
			NewHistogram(tc.name, gomock.Any(), bucketMatchers...).
			Do(func(_ string, _ string, buckets ...float64) {
				require.Equal(t, tc.buckets, buckets)
			}).
			Times(1)
	}

	c.registerFrameworkMetrics()
}

func TestGetDefaultDatasourceBuckets(t *testing.T) {
	buckets := getDefaultDatasourceBuckets()
	require.NotEmpty(t, buckets)

	assert.InDelta(t, 0.05, buckets[0], 1e-12)
	assert.InDelta(t, 30000.0, buckets[len(buckets)-1], 1e-12)

	for i := 1; i < len(buckets); i++ {
		assert.Greater(t, buckets[i], buckets[i-1])
	}
}

func TestContainer_Close_ClosesWebsocketConnections(t *testing.T) {
	c := &Container{
		WSManager: ws.New(),
	}

	connID := "conn-1"
	c.AddConnection(connID, &ws.Connection{})

	require.Len(t, c.WSManager.ListConnections(), 1)

	err := c.Close()
	require.NoError(t, err)

	require.Empty(t, c.WSManager.ListConnections())
}

// TestFrameworkMetricsSnapshot pins the set of framework metrics that
// GoFr registers by default: names, types, and HELP strings. These are
// the contract that user-built Grafana dashboards and alerts depend on.
//
// If a metric is added, renamed, or has its type/help changed, this test
// fails and the change has to be acknowledged by updating the expected
// list AND mentioning the dashboard impact in the PR body.
//
// Implementation: wires a fresh prometheus.Registry into an isolated
// MeterProvider so the global default registry (polluted by other tests
// in this package) does not interfere. Calls the production
// registerFrameworkMetrics() so the contract is asserted against real
// code, not a mirror.
//
//nolint:gocyclo,funlen // snapshot test enumerates each framework metric assertion linearly for diff-friendliness
func TestFrameworkMetricsSnapshot(t *testing.T) {
	reg := prometheus.NewRegistry()

	// Mirror exporters.Prometheus's configuration so the emitted names
	// match what users see (NoTranslation keeps OTel from rewriting
	// counter names that already end in _total etc).
	promExp, err := otelprom.New(
		otelprom.WithRegisterer(reg),
		otelprom.WithoutTargetInfo(),
		otelprom.WithTranslationStrategy(otlptranslator.NoTranslation),
	)
	require.NoError(t, err)

	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExp)).Meter("snapshot-test")

	c := &Container{
		appName:        "snapshot-test",
		appVersion:     "v0.0.0",
		Logger:         logging.NewMockLogger(logging.ERROR),
		metricsManager: metrics.NewMetricsManager(meter, logging.NewMockLogger(logging.ERROR)),
	}
	c.registerFrameworkMetrics()

	// OTel's Prometheus exporter only emits HELP/TYPE lines for metrics
	// that have at least one observation. Touch every framework metric
	// with a no-op observation so its definition appears on the wire.
	ctx := t.Context()

	for _, name := range []string{
		"app_http_retry_count",
		"app_pubsub_publish_total_count",
		"app_pubsub_publish_success_count",
		"app_pubsub_subscribe_total_count",
		"app_pubsub_subscribe_success_count",
	} {
		c.Metrics().IncrementCounter(ctx, name)
	}

	for _, name := range []string{
		"app_http_response",
		"app_http_service_response",
		"app_redis_stats",
		"app_sql_stats",
	} {
		c.Metrics().RecordHistogram(ctx, name, 0)
	}

	for _, name := range []string{
		"app_info",
		"app_go_routines",
		"app_sys_memory_alloc",
		"app_sys_total_alloc",
		"app_go_numGC",
		"app_go_sys",
		"app_http_circuit_breaker_state",
		"app_sql_open_connections",
		"app_sql_inUse_connections",
	} {
		c.Metrics().SetGauge(name, 0)
	}

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Extract the metric contract: pairs of "# HELP <name> <text>" and
	// "# TYPE <name> <type>". Skip value lines (they vary by run).
	got := map[string]struct {
		help string
		typ  string
	}{}

	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "# HELP "):
			parts := strings.SplitN(strings.TrimPrefix(line, "# HELP "), " ", 2)
			require.Len(t, parts, 2, "malformed HELP line: %q", line)

			entry := got[parts[0]]
			entry.help = parts[1]
			got[parts[0]] = entry
		case strings.HasPrefix(line, "# TYPE "):
			parts := strings.SplitN(strings.TrimPrefix(line, "# TYPE "), " ", 2)
			require.Len(t, parts, 2, "malformed TYPE line: %q", line)

			entry := got[parts[0]]
			entry.typ = parts[1]
			got[parts[0]] = entry
		}
	}

	want := map[string]struct {
		help string
		typ  string
	}{
		"app_info":                           {"Info for app_name, app_version and framework_version.", "gauge"},
		"app_go_routines":                    {"Number of Go routines running.", "gauge"},
		"app_sys_memory_alloc":               {"Number of bytes allocated for heap objects.", "gauge"},
		"app_sys_total_alloc":                {"Number of cumulative bytes allocated for heap objects.", "gauge"},
		"app_go_numGC":                       {"Number of completed Garbage Collector cycles.", "gauge"},
		"app_go_sys":                         {"Number of total bytes of memory.", "gauge"},
		"app_http_response":                  {"Response time of HTTP requests in seconds.", "histogram"},
		"app_http_service_response":          {"Response time of HTTP service requests in seconds.", "histogram"},
		"app_http_retry_count":               {"Total number of retry events", "counter"},
		"app_http_circuit_breaker_state":     {"Current state of the circuit breaker (0 for Closed, 1 for Open)", "gauge"},
		"app_redis_stats":                    {"Response time of Redis commands in milliseconds.", "histogram"},
		"app_sql_stats":                      {"Response time of SQL queries in milliseconds.", "histogram"},
		"app_sql_open_connections":           {"Number of open SQL connections.", "gauge"},
		"app_sql_inUse_connections":          {"Number of inUse SQL connections.", "gauge"},
		"app_pubsub_publish_total_count":     {"Number of total publish operations.", "counter"},
		"app_pubsub_publish_success_count":   {"Number of successful publish operations.", "counter"},
		"app_pubsub_subscribe_total_count":   {"Number of total subscribe operations.", "counter"},
		"app_pubsub_subscribe_success_count": {"Number of successful subscribe operations.", "counter"},
	}

	// First: assert that every framework metric is present with the
	// expected help + type. We allow extra metrics in the output (runtime
	// adds its own go_* metrics) — we just guard the GoFr contract.
	missing := []string{}
	mismatched := []string{}

	for name, exp := range want {
		actual, ok := got[name]
		if !ok {
			missing = append(missing, name)
			continue
		}

		if actual.help != exp.help || actual.typ != exp.typ {
			mismatched = append(mismatched,
				fmt.Sprintf("%s: want (help=%q, type=%q) got (help=%q, type=%q)",
					name, exp.help, exp.typ, actual.help, actual.typ))
		}
	}

	sort.Strings(missing)
	sort.Strings(mismatched)

	assert.Empty(t, missing, "framework metrics missing from /metrics output")
	assert.Empty(t, mismatched, "framework metric definitions drifted")
}
