package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

// barrierTimeout bounds how long a check waits at the barrier. It only elapses when the checks are
// not actually concurrent, which is the failure the barrier exists to catch.
const barrierTimeout = 5 * time.Second

// checkBarrier releases only once n checks are inside it at the same moment. A sequential
// implementation can never assemble n of them, so the barrier times out and the test reports it
// rather than hanging the suite.
//
// It proves concurrency by construction, which a wall-clock upper bound does not: a loaded CI box
// makes this assertion easier to satisfy, never harder.
type checkBarrier struct {
	pending  atomic.Int64
	release  chan struct{}
	timedOut atomic.Bool
}

func newCheckBarrier(n int) *checkBarrier {
	b := &checkBarrier{release: make(chan struct{})}
	b.pending.Store(int64(n))

	return b
}

func (b *checkBarrier) arrive() {
	if b.pending.Add(-1) == 0 {
		close(b.release)
	}

	select {
	case <-b.release:
	case <-time.After(barrierTimeout):
		b.timedOut.Store(true)
	}
}

// expectHealthyBackends sets an UP expectation on every backend a mock container carries. hook runs
// inside the Redis, PubSub and Mongo checks — one from each of the three fan-out groups — so a test
// can observe those three running together.
//
// The SQL mock queues its health results, pops one per call and asserts at cleanup that the queue
// came out empty, so rounds must be exactly the number of check rounds the test performs.
func expectHealthyBackends(mocks *Mocks, rounds int, hook func()) {
	up := datasource.Health{Status: datasource.StatusUp}

	for range rounds {
		mocks.SQL.ExpectHealthCheck().WillReturnHealthCheck(&datasource.Health{Status: datasource.StatusUp})
	}

	mocks.Redis.EXPECT().HealthCheck().DoAndReturn(func() datasource.Health {
		hook()

		return up
	}).AnyTimes()

	mocks.PubSub.EXPECT().Health().DoAndReturn(func() datasource.Health {
		hook()

		return up
	}).AnyTimes()

	mocks.Mongo.EXPECT().HealthCheck(gomock.Any()).DoAndReturn(func(context.Context) (any, error) {
		hook()

		return up, nil
	}).AnyTimes()

	mocks.Cassandra.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.Clickhouse.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.Oracle.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.KVStore.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.DGraph.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.OpenTSDB.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.Elasticsearch.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
	mocks.Couchbase.EXPECT().HealthCheck(gomock.Any()).Return(up, nil).AnyTimes()
}

// The point of the change: checks from all three fan-out groups are in flight at once. Sequential
// execution cannot fill the barrier and the test says so.
func TestContainer_Health_ChecksRunConcurrently(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"

	barrier := newCheckBarrier(3)
	expectHealthyBackends(mocks, 1, barrier.arrive)

	result, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.False(t, barrier.timedOut.Load(),
		"redis, pubsub and mongo never ran at the same time — the checks are still sequential")
	assert.Equal(t, datasource.StatusUp, result["status"])
}

// Concurrent probes must collapse into one round of checks, so a slow backend cannot be stampeded
// by probe traffic. This is what the cache alone does not give: it only helps after a round has
// already finished.
func TestContainer_Health_CollapsesConcurrentCalls(t *testing.T) {
	const callers = 8

	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(0, 0) // caching off, so only the singleflight can collapse these

	// The SQL mock queues a fixed number of results and panics past the end. Dropping it keeps a
	// failure here reported as the round count it is, rather than as a nil dereference.
	c.SQL = nil

	var (
		rounds  atomic.Int64
		entered atomic.Int64
	)

	// The leading round holds until every caller has entered Health and had a moment to reach the
	// singleflight, so a straggler cannot start a second round for reasons of scheduling.
	expectHealthyBackends(mocks, 0, func() {
		if rounds.Add(1) != 1 {
			return
		}

		deadline := time.Now().Add(barrierTimeout)
		for entered.Load() < callers && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}

		time.Sleep(50 * time.Millisecond)
	})

	var wg sync.WaitGroup

	results := make([]map[string]any, callers)

	for i := range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			entered.Add(1)

			results[i], _ = c.Health(t.Context()).(map[string]any)
		}()
	}

	wg.Wait()

	// Three hooked backends, one round each.
	assert.Equal(t, int64(3), rounds.Load(), "%d concurrent calls ran more than one round of checks", callers)

	for i, result := range results {
		assert.Equal(t, datasource.StatusUp, result["status"], "caller %d got no result", i)
	}
}

// Every caller must get its own map. Handing out the cached one lets any consumer mutate what the
// next caller reads, and race the reader while doing it.
func TestContainer_Health_ReturnsIndependentCopies(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(time.Minute, 0)

	expectHealthyBackends(mocks, 1, func() {})

	first, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	first["status"] = "TAMPERED"
	delete(first, "redis")

	second, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, datasource.StatusUp, second["status"], "a caller mutated the cached result")
	assert.Contains(t, second, "redis", "a caller deleted a key from the cached result")
}

// A cache hit must not touch the backends at all.
func TestContainer_Health_ServesFromCacheWithinTTL(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(time.Minute, 0)

	var checks atomic.Int64

	expectHealthyBackends(mocks, 1, func() { checks.Add(1) })

	for range 5 {
		result, ok := c.Health(t.Context()).(map[string]any)
		require.True(t, ok)
		assert.Equal(t, datasource.StatusUp, result["status"])
	}

	assert.Equal(t, int64(3), checks.Load(), "the cache did not spare the backends on repeat calls")
}

// An expired entry must fall through to a fresh round rather than being served forever.
func TestContainer_Health_RechecksAfterTTL(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(time.Minute, 0)

	var checks atomic.Int64

	expectHealthyBackends(mocks, 2, func() { checks.Add(1) })

	c.Health(t.Context())

	// Age the entry past its TTL instead of waiting one out.
	c.health.mu.Lock()
	c.health.storedAt = time.Now().Add(-time.Hour)
	c.health.mu.Unlock()

	c.Health(t.Context())

	assert.Equal(t, int64(6), checks.Load(), "an expired entry was served instead of re-checking")
}

// Caching is off unless configured, so an upgrade never silently starts serving a stale body.
func TestContainer_Health_NotCachedByDefault(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(
		c.healthDuration(healthConfig{}, healthCacheTTLKey),
		c.healthDuration(healthConfig{}, healthCheckTimeoutKey),
	)

	var checks atomic.Int64

	expectHealthyBackends(mocks, 2, func() { checks.Add(1) })

	c.Health(t.Context())
	c.Health(t.Context())

	assert.Equal(t, int64(6), checks.Load(), "health was cached without HEALTH_CACHE_TTL being set")
}

// A Container that never went through Create has no probe. The checks still have to run.
func TestContainer_Health_WithoutProbe(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"

	require.Nil(t, c.health, "NewMockContainer is expected to leave the probe unset")

	expectHealthyBackends(mocks, 1, func() {})

	result, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, datasource.StatusUp, result["status"])
	assert.Contains(t, result, "redis")
}

// HEALTH_CHECK_TIMEOUT bounds one round. What did not answer is missing from the body, and the
// aggregate is DEGRADED rather than a confident UP built out of a partial map.
func TestContainer_Health_TimesOutSlowBackend(t *testing.T) {
	released := make(chan struct{})

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-released
		w.WriteHeader(http.StatusOK)
	}))

	// Close releases the handler first, so the abandoned check finishes before the server goes.
	t.Cleanup(func() {
		close(released)
		slow.Close()
	})

	logger := logging.NewMockLogger(logging.ERROR)

	c := &Container{
		Logger:     logger,
		appName:    "test-app",
		appVersion: "test",
		health:     newHealthProbe(time.Minute, 50*time.Millisecond),
		Services: map[string]service.HTTP{
			"slow-service": service.NewHTTPService(slow.URL, logger, nil),
		},
	}

	result, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, datasource.StatusDegraded, result["status"], "a timed-out round must not report UP")
	assert.NotContains(t, result, "slow-service", "a backend that never answered must not appear")

	_, cached := c.health.get()
	assert.False(t, cached, "a partial result must never be cached")
}

// With no timeout configured the round is bounded only by each backend client's own timeout, which
// is how GoFr has always behaved.
func TestContainer_Health_NoTimeoutByDefault(t *testing.T) {
	c, mocks := NewMockContainer(t)
	c.appName, c.appVersion = "test-app", "test"
	c.health = newHealthProbe(0, 0)

	expectHealthyBackends(mocks, 1, func() { time.Sleep(100 * time.Millisecond) })

	result, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, datasource.StatusUp, result["status"], "a slow but successful round was cut short")
	assert.Contains(t, result, "redis")
}

// A caller disconnecting must not cancel the round of checks — the other waiters on the
// singleflight are sharing it, and even alone a half-finished map is worse than a slow one. Both
// paths through Health have to hold to that, including the one without a probe.
func TestContainer_Health_SurvivesCallerCancellation(t *testing.T) {
	tests := []struct {
		desc  string
		probe *healthProbe
	}{
		{"with a probe", newHealthProbe(0, 0)},
		{"without a probe", nil},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c, mocks := NewMockContainer(t)
			c.appName, c.appVersion = "test-app", "test"
			c.health = tc.probe

			ctx, cancel := context.WithCancel(t.Context())

			// The calling request goes away while the round is still running.
			expectHealthyBackends(mocks, 1, cancel)

			result, ok := c.Health(ctx).(map[string]any)
			require.True(t, ok)

			assert.Equal(t, datasource.StatusUp, result["status"],
				"canceling the calling request aborted the round of checks")
			assert.Contains(t, result, "mongo",
				"a context-aware check was canceled by the caller going away")
		})
	}
}

// Models added with AddLLM report alongside the datasources: the default under "llm", a named one
// under "llm_<name>". A model that is down degrades the aggregate like any other dependency.
func TestContainer_Health_IncludesLLMModels(t *testing.T) {
	tests := []struct {
		desc     string
		status   string
		expected string
	}{
		{"a healthy model", datasource.StatusUp, datasource.StatusUp},
		{"a model that is down", datasource.StatusDown, datasource.StatusDegraded},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c, mocks := NewMockContainer(t)
			c.appName, c.appVersion = "test-app", "test"

			ctrl := gomock.NewController(t)

			defaultModel := ai.NewMockModel(ctrl)
			defaultModel.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{Status: tc.status})

			namedModel := ai.NewMockModel(ctrl)
			namedModel.EXPECT().HealthCheck(gomock.Any()).Return(datasource.Health{Status: datasource.StatusUp})

			c.llmModels = map[string]ai.Model{"": defaultModel, "claude": namedModel}

			expectHealthyBackends(mocks, 1, func() {})

			result, ok := c.Health(t.Context()).(map[string]any)
			require.True(t, ok)

			assert.Equal(t, datasource.Health{Status: tc.status}, result[llmKey])
			assert.Equal(t, datasource.Health{Status: datasource.StatusUp}, result["llm_claude"])
			assert.Equal(t, tc.expected, result["status"])
		})
	}
}

// panicHTTPService is a service.HTTP whose health check panics, standing in for a driver that
// panics on a malformed response or a nil dependency.
type panicHTTPService struct {
	service.HTTP
}

func (*panicHTTPService) HealthCheck(context.Context) *service.Health {
	panic("boom")
}

// A panicking backend used to be contained by the handler's panic-recovery middleware, because the
// checks ran inline on the request goroutine. Running them concurrently moved them out from under
// it, where the same panic takes the process down instead of one request.
func TestContainer_Health_PanickingBackendIsReportedDown(t *testing.T) {
	c := &Container{
		Logger:     logging.NewMockLogger(logging.ERROR),
		appName:    "test-app",
		appVersion: "test",
		health:     newHealthProbe(0, 0),
		Services: map[string]service.HTTP{
			"panicking-service": &panicHTTPService{},
		},
	}

	// The assertion is as much that this line is reached at all: without the recover the panic
	// unwinds a goroutine of ours and the test binary dies rather than failing.
	result, ok := c.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, datasource.StatusDegraded, result["status"], "a panicking backend must not report UP")

	reported, ok := result["panicking-service"].(datasource.Health)
	require.True(t, ok, "the panicking backend must still appear in the body")
	assert.Equal(t, datasource.StatusDown, reported.Status)
	assert.Contains(t, reported.Details["error"], "panicked")
}

// After a timeout the outstanding checks are abandoned rather than canceled, so they are still
// running when the next probe arrives. That probe must not start a second set against the same
// unanswering backend: under Kubernetes probe traffic that is one more leaked goroutine every
// probe interval, for as long as the backend stays hung.
func TestContainer_Health_DoesNotStackRoundsAfterTimeout(t *testing.T) {
	var started atomic.Int64

	released := make(chan struct{})

	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started.Add(1)
		<-released
		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		close(released)
		hung.Close()
	})

	logger := logging.NewMockLogger(logging.ERROR)

	c := &Container{
		Logger:     logger,
		appName:    "test-app",
		appVersion: "test",
		health:     newHealthProbe(0, 50*time.Millisecond),
		Services: map[string]service.HTTP{
			"hung-service": service.NewHTTPService(hung.URL, logger, nil),
		},
	}

	// Several sequential probes, each of which times out. Sequential rather than concurrent on
	// purpose: singleflight already collapses concurrent callers, so only separate rounds can
	// expose the stacking.
	const probes = 4

	for range probes {
		result, ok := c.Health(t.Context()).(map[string]any)
		require.True(t, ok)
		assert.Equal(t, datasource.StatusDegraded, result["status"])
	}

	assert.Equal(t, int64(1), started.Load(),
		"each probe started another check against a backend that had not answered the first")
}

// The in-flight guard must not be a one-way door. Once the abandoned checks finally answer, the
// claim is released and the next probe runs a fresh round — otherwise a single slow backend would
// pin the endpoint to DEGRADED for the life of the process.
func TestContainer_Health_RecoversAfterHungBackendAnswers(t *testing.T) {
	release := make(chan struct{})

	var calls atomic.Int64

	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hung.Close)

	logger := logging.NewMockLogger(logging.ERROR)

	c := &Container{
		Logger:     logger,
		appName:    "test-app",
		appVersion: "test",
		health:     newHealthProbe(0, 50*time.Millisecond),
		Services: map[string]service.HTTP{
			"hung": service.NewHTTPService(hung.URL, logger, nil),
		},
	}

	for i := range 3 {
		m, ok := c.Health(t.Context()).(map[string]any)
		require.True(t, ok)
		require.Equal(t, datasource.StatusDegraded, m["status"], "probe %d while the backend hung", i)
	}

	require.Equal(t, int64(1), calls.Load(), "rounds stacked while the backend was hung")

	close(release)

	require.Eventually(t, func() bool {
		m, ok := c.Health(t.Context()).(map[string]any)

		return ok && m["status"] == datasource.StatusUp
	}, 10*time.Second, 50*time.Millisecond,
		"health never recovered after the hung backend answered — the round claim leaked")
}

// The point of the in-flight guard is that goroutines do not accumulate. Counting them directly is
// the only assertion that actually proves it: the round count could stay at one while each timed-out
// round still left a waiter behind.
func TestContainer_Health_TimeoutsDoNotAccumulateGoroutines(t *testing.T) {
	release := make(chan struct{})

	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		close(release)
		hung.Close()
	})

	logger := logging.NewMockLogger(logging.ERROR)

	c := &Container{
		Logger:     logger,
		appName:    "test-app",
		appVersion: "test",
		health:     newHealthProbe(0, 20*time.Millisecond),
		Services: map[string]service.HTTP{
			"hung": service.NewHTTPService(hung.URL, logger, nil),
		},
	}

	// One probe first, so the goroutines a single timed-out round legitimately keeps — the
	// abandoned check and the waiter that releases the claim — are already in the baseline.
	_ = c.Health(t.Context())

	time.Sleep(50 * time.Millisecond)

	baseline := runtime.NumGoroutine()

	for range 25 {
		_ = c.Health(t.Context())
	}

	time.Sleep(50 * time.Millisecond)

	growth := runtime.NumGoroutine() - baseline

	// Without the guard each probe leaves behind its own check plus a waiter, so 25 probes grow
	// the count by roughly 50. A couple of goroutines of slack absorbs the runtime's own churn.
	assert.LessOrEqual(t, growth, 5,
		"25 timed-out probes grew the goroutine count by %d; rounds are accumulating", growth)

	t.Logf("25 timed-out probes: goroutine growth %d (baseline %d)", growth, baseline)
}
