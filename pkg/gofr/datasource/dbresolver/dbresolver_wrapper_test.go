package dbresolver

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/container"
)

// Mocks contains all the dependencies needed for testing.
type Mocks struct {
	Ctrl         *gomock.Controller
	Primary      *MockDB
	Replicas     []container.DB
	MockReplicas []*MockDB
	Strategy     *MockStrategy
	Logger       Logger
	Metrics      Metrics
	Resolver     *Resolver
}

// setupMocks creates and returns common mocks used in tests.
func setupMocks(t *testing.T, createResolver bool) *Mocks {
	t.Helper()

	// Create controller and mocks.
	ctrl := gomock.NewController(t)
	mockPrimary := NewMockDB(ctrl)
	mockReplica1 := NewMockDB(ctrl)
	mockReplica2 := NewMockDB(ctrl)

	// Create both typed and interface versions of replicas.
	typedMockReplicas := []*MockDB{mockReplica1, mockReplica2}

	mockReplicas := make([]container.DB, len(typedMockReplicas))
	for i, r := range typedMockReplicas {
		mockReplicas[i] = r
	}

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockStrategy := NewMockStrategy(ctrl)

	mockMetrics.EXPECT().NewHistogram("dbresolver_query_duration",
		"Response time of DB resolver operations in microseconds",
		gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks := &Mocks{
		Ctrl:         ctrl,
		Primary:      mockPrimary,
		Replicas:     mockReplicas,
		MockReplicas: typedMockReplicas,
		Strategy:     mockStrategy,
		Logger:       mockLogger,
		Metrics:      mockMetrics,
	}

	if createResolver {
		replicaWrappers := make([]*replicaWrapper, len(mockReplicas))
		for i, r := range mockReplicas {
			replicaWrappers[i] = &replicaWrapper{
				db:      r,
				breaker: newCircuitBreaker(5, 30*time.Second),
			}
		}

		mocks.Resolver = &Resolver{
			primary:      mockPrimary,
			replicas:     replicaWrappers,
			strategy:     mockStrategy,
			readFallback: true,
			tracer:       otel.GetTracerProvider().Tracer("gofr-dbresolver"),
			logger:       mockLogger,
			metrics:      mockMetrics,
			stats:        &statistics{},
			stopChan:     make(chan struct{}),
			once:         sync.Once{},
			queryCache:   newQueryCache(100),
		}
	}

	return mocks
}

func TestResolverWrapper_Build_Error(t *testing.T) {
	mocks := setupMocks(t, false)
	defer mocks.Ctrl.Finish()

	t.Run("Error_NilPrimary", func(t *testing.T) {
		wrapper := NewProvider(NewRoundRobinStrategy(), true)
		wrapper.UseLogger(mocks.Logger)
		wrapper.UseMetrics(mocks.Metrics)
		wrapper.UseTracer(otel.GetTracerProvider().Tracer("gofr-dbresolver"))

		resolver, err := wrapper.Build(nil, mocks.Replicas)

		require.Error(t, err)
		assert.Equal(t, errPrimaryNil.Error(), err.Error())
		assert.Nil(t, resolver)
	})
}

func TestResolverWrapper_Build_Success(t *testing.T) {
	mocks := setupMocks(t, false)
	defer mocks.Ctrl.Finish()

	successTests := []struct {
		name         string
		strategy     Strategy
		readFallback bool
		replicas     []container.DB
	}{
		{
			name:         "RoundRobinStrategy",
			strategy:     NewRoundRobinStrategy(),
			readFallback: true,
			replicas:     mocks.Replicas,
		},
		{
			name:         "RandomStrategy",
			strategy:     NewRandomStrategy(),
			readFallback: false,
			replicas:     mocks.Replicas,
		},
		{
			name:         "EmptyReplicas",
			strategy:     NewRoundRobinStrategy(),
			readFallback: true,
			replicas:     []container.DB{},
		},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := NewProvider(tt.strategy, tt.readFallback)
			wrapper.UseLogger(mocks.Logger)
			wrapper.UseMetrics(mocks.Metrics)

			resolver, err := wrapper.Build(mocks.Primary, tt.replicas)

			require.NoError(t, err)
			assert.NotNil(t, resolver)

			_, ok := resolver.(*Resolver)
			assert.True(t, ok, "Result should be a *Resolver")
		})
	}
}
