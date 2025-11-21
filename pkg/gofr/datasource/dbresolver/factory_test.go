package dbresolver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
)

func TestNewDBResolverProvider(t *testing.T) {
	app := &gofr.App{}
	cfg := Config{
		Strategy:     StrategyRoundRobin,
		ReadFallback: true,
	}

	provider := NewDBResolverProvider(app, cfg)

	require.NotNil(t, provider)
	assert.Equal(t, app, provider.app)
	assert.Equal(t, cfg, provider.cfg)
}

func TestResolverProvider_GetResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := NewMockDB(ctrl)
	provider := &ResolverProvider{
		resolver: mockDB,
	}

	resolver := provider.GetResolver()

	assert.Equal(t, mockDB, resolver)
}

func TestCreateStrategy_RoundRobin(t *testing.T) {
	strategy := getStrategy(StrategyRoundRobin)

	assert.NotNil(t, strategy)
	assert.IsType(t, &RoundRobinStrategy{}, strategy)
}

func TestCreateStrategy_Random(t *testing.T) {
	strategy := getStrategy(StrategyRandom)

	assert.NotNil(t, strategy)
	assert.IsType(t, &RandomStrategy{}, strategy)
}

func TestCreateStrategy_Default(t *testing.T) {
	strategy := getStrategy(StrategyType("unknown"))

	assert.NotNil(t, strategy)
	assert.IsType(t, &RoundRobinStrategy{}, strategy)
}

func TestReplicaConfig_Get(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_NAME": "testdb",
	})

	replicaCfg := &replicaConfig{
		base:     mockConfig,
		host:     "localhost",
		port:     "3307",
		user:     "replica_user",
		password: "replica_pass",
	}

	assert.Equal(t, "localhost", replicaCfg.Get("DB_HOST"))
	assert.Equal(t, "3307", replicaCfg.Get("DB_PORT"))
	assert.Equal(t, "replica_user", replicaCfg.Get("DB_USER"))
	assert.Equal(t, "replica_pass", replicaCfg.Get("DB_PASSWORD"))
	assert.Equal(t, "testdb", replicaCfg.Get("DB_NAME"))
}

func TestReplicaConfig_GetOrDefault(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{})

	replicaCfg := &replicaConfig{
		base: mockConfig,
	}

	result := replicaCfg.GetOrDefault("EMPTY_KEY", "default_value")

	assert.Equal(t, "default_value", result)
}

func TestReplicaConfig_GetMaxIdleConnections(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_IDLE_CONNECTIONS": "5",
	})

	replicaCfg := &replicaConfig{
		base: mockConfig,
	}

	result := replicaCfg.Get("DB_MAX_IDLE_CONNECTIONS")

	assert.Equal(t, "5", result)
}

func TestReplicaConfig_GetMaxOpenConnections(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_OPEN_CONNECTIONS": "10",
	})

	replicaCfg := &replicaConfig{
		base: mockConfig,
	}

	result := replicaCfg.Get("DB_MAX_OPEN_CONNECTIONS")

	assert.Equal(t, "10", result)
}

func TestGetReplicaConfigInt_ValidValue(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"TEST_KEY": "42",
	})

	result := getReplicaConfigInt(mockConfig, "TEST_KEY", 10)

	assert.Equal(t, 42, result)
}

func TestGetReplicaConfigInt_EmptyValue(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{})

	result := getReplicaConfigInt(mockConfig, "MISSING_KEY", 10)

	assert.Equal(t, 10, result)
}

func TestGetReplicaConfigInt_InvalidValue(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"TEST_KEY": "invalid",
	})

	result := getReplicaConfigInt(mockConfig, "TEST_KEY", 10)

	assert.Equal(t, 10, result)
}

func TestOptimizedIdleConnections(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_IDLE_CONNECTIONS": "5",
	})

	result := optimizedIdleConnections(mockConfig)

	assert.Equal(t, "5", result)
}

func TestOptimizedIdleConnections_BelowMin(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_IDLE_CONNECTIONS": "1",
	})

	result := optimizedIdleConnections(mockConfig)

	assert.Equal(t, "2", result)
}

func TestOptimizedIdleConnections_AboveMax(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_IDLE_CONNECTIONS": "15",
	})

	result := optimizedIdleConnections(mockConfig)

	assert.Equal(t, "10", result)
}

func TestOptimizedOpenConnections(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_OPEN_CONNECTIONS": "8",
	})

	result := optimizedOpenConnections(mockConfig)

	assert.Equal(t, "8", result)
}

func TestOptimizedOpenConnections_BelowMin(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_OPEN_CONNECTIONS": "2",
	})

	result := optimizedOpenConnections(mockConfig)

	assert.Equal(t, "5", result)
}

func TestOptimizedOpenConnections_AboveMax(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_MAX_OPEN_CONNECTIONS": "25",
	})

	result := optimizedOpenConnections(mockConfig)

	assert.Equal(t, "20", result)
}

func TestCreateReplicas_EmptyConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := config.NewMockConfig(map[string]string{})
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	replicas, err := connectReplicas(mockConfig, mockLogger, mockMetrics)

	require.NoError(t, err)
	assert.Nil(t, replicas)
}

func TestCreateReplicas_InvalidHostFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_HOSTS": "invalid-host",
		"DB_USER":          "user",
		"DB_PASSWORD":      "pass",
	})
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	replicas, err := connectReplicas(mockConfig, mockLogger, mockMetrics)

	require.Error(t, err)
	assert.Nil(t, replicas)
	assert.ErrorIs(t, err, errInvalidReplicaHostFormat)
}

func TestCreateReplicaConnection_NoDBName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := config.NewMockConfig(map[string]string{})
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	replica, err := createReplicaConnection(mockConfig, "host", "3306", "user", "pass", mockLogger, mockMetrics)

	require.Error(t, err)
	assert.Nil(t, replica)
	assert.ErrorIs(t, err, errDBNameRequired)
}

func TestResolverProvider_UseTracer(t *testing.T) {
	provider := &ResolverProvider{}

	tracer := noop.NewTracerProvider().Tracer("test")
	provider.UseTracer(tracer)

	assert.Equal(t, tracer, provider.tracer)
}

func TestResolverProvider_UseTracer_InvalidType(t *testing.T) {
	provider := &ResolverProvider{}

	provider.UseTracer("invalid")

	assert.Nil(t, provider.tracer)
}

func TestResolverProvider_Connect_InvalidLogger(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{})

	app := gofr.New()
	app.Config = mockConfig

	provider := &ResolverProvider{
		app:    app,
		logger: "invalid-logger", // Wrong type
	}

	provider.Connect()

	assert.Nil(t, provider.resolver)
}

func TestResolverProvider_Connect_InvalidMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{})

	app := gofr.New()
	app.Config = mockConfig

	mockLogger.EXPECT().Errorf("Invalid logger or metrics type").Times(1)

	provider := &ResolverProvider{
		app:     app,
		logger:  mockLogger,
		metrics: "invalid-metrics", // Wrong type
	}

	provider.Connect()

	assert.Nil(t, provider.resolver)
}

func TestResolverProvider_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	provider := &ResolverProvider{}

	provider.UseLogger(mockLogger)

	assert.Equal(t, mockLogger, provider.logger)
}

func TestResolverProvider_UseMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)
	provider := &ResolverProvider{}

	provider.UseMetrics(mockMetrics)

	assert.Equal(t, mockMetrics, provider.metrics)
}

func TestResolverProvider_GetResolver_Nil(t *testing.T) {
	provider := &ResolverProvider{}

	resolver := provider.GetResolver()

	assert.Nil(t, resolver)
}

func TestCreateReplicas_NoReplicaHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{})

	replicas, err := connectReplicas(mockConfig, mockLogger, mockMetrics)

	require.NoError(t, err)
	assert.Nil(t, replicas)
}

func TestCreateReplicas_EmptyHostAfterTrim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{
		"DB_REPLICA_HOSTS": "  ,  , localhost:3307",
		"DB_NAME":          "testdb",
	})

	replicas, err := connectReplicas(mockConfig, mockLogger, mockMetrics)

	require.Error(t, err)
	assert.Nil(t, replicas)
}

func TestCreateHTTPMiddleware(t *testing.T) {
	middleware := createHTTPMiddleware()

	assert.NotNil(t, middleware)

	handlerCalled := false
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		method := r.Context().Value(contextKeyHTTPMethod)
		path := r.Context().Value(contextKeyRequestPath)

		assert.Equal(t, "GET", method)
		assert.Equal(t, "/test/path", path)
	})

	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test/path", http.NoBody)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
}

func TestCircuitBreaker_AllowRequest_StateOpen_WithinTimeout(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Set state to open
	openState := circuitStateOpen
	cb.state.Store(&openState)

	// Record a failure to set lastFailure
	now := time.Now()
	cb.lastFailure.Store(&now)

	// Should not allow request (within timeout)
	result := cb.allowRequest()

	assert.False(t, result)
	assert.Equal(t, circuitStateOpen, *cb.state.Load())
}

func TestCircuitBreaker_AllowRequest_StateOpen_NilLastFailure(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Set state to open but no last failure
	openState := circuitStateOpen
	cb.state.Store(&openState)
	cb.lastFailure.Store(nil)

	// Should allow request when lastFailure is nil
	result := cb.allowRequest()

	assert.True(t, result)
}

func TestCircuitBreaker_RecordFailure_OpensCircuit(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Record failures up to threshold
	cb.recordFailure() // 1
	assert.Equal(t, circuitStateClosed, *cb.state.Load())

	cb.recordFailure() // 2
	assert.Equal(t, circuitStateClosed, *cb.state.Load())

	cb.recordFailure() // 3 - should open circuit
	assert.Equal(t, circuitStateOpen, *cb.state.Load())
	assert.Equal(t, int32(3), cb.failures.Load())
}

func TestCircuitBreaker_RecordFailure_LastFailureUpdated(t *testing.T) {
	cb := newCircuitBreaker(5, 50*time.Millisecond)

	before := time.Now()

	cb.recordFailure()

	after := time.Now()

	lastFailurePtr := cb.lastFailure.Load()
	require.NotNil(t, lastFailurePtr)

	assert.True(t, lastFailurePtr.After(before) || lastFailurePtr.Equal(before))
	assert.True(t, lastFailurePtr.Before(after) || lastFailurePtr.Equal(after))
}

func TestCircuitBreaker_AllowRequest_HalfOpenState(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Set state to half-open
	halfOpenState := circuitStateHalfOpen
	cb.state.Store(&halfOpenState)

	// Should allow request in half-open state
	result := cb.allowRequest()

	assert.True(t, result)
}

func TestCircuitBreaker_RecordSuccess_ResetsCircuit(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Record some failures
	cb.recordFailure()
	cb.recordFailure()

	// Record success
	cb.recordSuccess()

	// Should reset everything
	assert.Equal(t, int32(0), cb.failures.Load())
	assert.Nil(t, cb.lastFailure.Load())
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
}

func TestCircuitBreaker_AllowRequest_StateOpen_AfterTimeout(t *testing.T) {
	cb := newCircuitBreaker(3, 30*time.Millisecond)

	// Trigger circuit to open by recording failures
	cb.recordFailure()
	cb.recordFailure()
	cb.recordFailure()

	// Verify it's open
	assert.Equal(t, circuitStateOpen, *cb.state.Load())

	// Capture the CURRENT state pointer (the one set by recordFailure)
	currentStatePtr := cb.state.Load()

	// Simulate timeout by setting lastFailure in the past
	pastTime := time.Now().Add(-50 * time.Millisecond)
	cb.lastFailure.Store(&pastTime)

	// Timeout has passed - CompareAndSwap will work because we're using the same pointer
	halfOpenState := circuitStateHalfOpen
	swapped := cb.state.CompareAndSwap(currentStatePtr, &halfOpenState)
	assert.True(t, swapped)

	// Now verify allowRequest works correctly
	result := cb.allowRequest()
	assert.True(t, result)
	assert.Equal(t, circuitStateHalfOpen, *cb.state.Load())
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := newCircuitBreaker(2, 30*time.Millisecond)

	// Start closed
	assert.Equal(t, circuitStateClosed, *cb.state.Load())

	// Record failures to open
	cb.recordFailure()
	cb.recordFailure()
	assert.Equal(t, circuitStateOpen, *cb.state.Load())

	// Capture current state pointer
	currentStatePtr := cb.state.Load()

	// Simulate timeout by setting lastFailure in the past
	pastTime := time.Now().Add(-50 * time.Millisecond)
	cb.lastFailure.Store(&pastTime)

	// Manually transition to half-open using the captured pointer
	halfOpenState := circuitStateHalfOpen
	swapped := cb.state.CompareAndSwap(currentStatePtr, &halfOpenState)
	assert.True(t, swapped)

	// Verify state changed
	assert.Equal(t, circuitStateHalfOpen, *cb.state.Load())

	// Record success to close
	cb.recordSuccess()
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
}

func TestCircuitBreaker_AllowRequest_ClosedState(t *testing.T) {
	cb := newCircuitBreaker(3, 50*time.Millisecond)

	// Should allow request in closed state
	result := cb.allowRequest()

	assert.True(t, result)
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
}

func TestCircuitBreaker_MultipleFailuresUnderThreshold(t *testing.T) {
	cb := newCircuitBreaker(5, 50*time.Millisecond)

	// Record failures under threshold
	cb.recordFailure()
	cb.recordFailure()
	cb.recordFailure()

	// Should still be closed
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
	assert.Equal(t, int32(3), cb.failures.Load())
}

func TestNewCircuitBreaker(t *testing.T) {
	maxFailures := int32(5)
	timeout := 50 * time.Millisecond

	cb := newCircuitBreaker(maxFailures, timeout)

	require.NotNil(t, cb)
	assert.Equal(t, maxFailures, cb.maxFailures)
	assert.Equal(t, timeout, cb.timeout)
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
	assert.Equal(t, int32(0), cb.failures.Load())
}

func TestCircuitBreaker_ConcurrentFailures(t *testing.T) {
	cb := newCircuitBreaker(10, 50*time.Millisecond)

	// Record multiple failures concurrently
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			cb.recordFailure()

			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify failures were recorded
	failures := cb.failures.Load()
	assert.Equal(t, int32(5), failures)
	assert.Equal(t, circuitStateClosed, *cb.state.Load())
}
