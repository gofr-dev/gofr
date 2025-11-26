package dbresolver

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

var errTestReplicaFailed = errors.New("replica connection failed")

const (
	healthStatusUP   = "UP"
	healthStatusDOWN = "DOWN"
)

// Mocks contains all the dependencies needed for testing.
type Mocks struct {
	Ctrl         *gomock.Controller
	Primary      *MockDB
	Replicas     []container.DB
	MockReplicas []*MockDB
	Strategy     *MockStrategy
	Logger       MockLogger
	Metrics      Metrics
	Resolver     *Resolver
}

// setupMocks creates and returns common mocks used in tests.
func setupMocks(t *testing.T) *Mocks {
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
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks := &Mocks{
		Ctrl:         ctrl,
		Primary:      mockPrimary,
		Replicas:     mockReplicas,
		MockReplicas: typedMockReplicas,
		Strategy:     mockStrategy,
		Logger:       *mockLogger,
		Metrics:      mockMetrics,
	}

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
	}

	return mocks
}

// MockResult is a mock of sql.Result interface.
type MockResult struct {
	ctrl     *gomock.Controller
	recorder *MockResultMockRecorder
}

// MockResultMockRecorder is the mock recorder for MockResult.
type MockResultMockRecorder struct {
	mock *MockResult
}

// NewMockResult creates a new mock instance.
func NewMockResult(ctrl *gomock.Controller) *MockResult {
	mock := &MockResult{ctrl: ctrl}
	mock.recorder = &MockResultMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResult) EXPECT() *MockResultMockRecorder {
	return m.recorder
}

// LastInsertId mocks base method.
func (m *MockResult) LastInsertId() (int64, error) {
	ret := m.ctrl.Call(m, "LastInsertId")

	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// LastInsertId indicates an expected call of LastInsertId.
func (mr *MockResultMockRecorder) LastInsertId() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LastInsertId", reflect.TypeOf((*MockResult)(nil).LastInsertId))
}

// RowsAffected mocks base method.
func (m *MockResult) RowsAffected() (int64, error) {
	ret := m.ctrl.Call(m, "RowsAffected")
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// RowsAffected indicates an expected call of RowsAffected.
func (mr *MockResultMockRecorder) RowsAffected() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RowsAffected", reflect.TypeOf((*MockResult)(nil).RowsAffected))
}

func TestResolver_Query_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	ctx := WithHTTPMethod(t.Context(), "GET")

	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(ctx, readQuery)

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_Query_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	writeQuery := "INSERT INTO users (name) VALUES (?)"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryContext(gomock.Any(), writeQuery, args[0]).Return(expectedRows, nil)

	rows, err := mocks.Resolver.Query(writeQuery, args[0])

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryContext_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	ctx := WithHTTPMethod(t.Context(), "GET")

	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery, args[0]).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(ctx, readQuery, args[0])

	require.NoError(t, err)
	require.NoError(t, rows.Err())
	assert.NotNil(t, rows)
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryContext_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	writeQuery := "INSERT INTO users (name) VALUES (?)"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryContext(gomock.Any(), writeQuery, args[0]).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(t.Context(), writeQuery, args[0])

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryRow_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	readQuery := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	ctx := WithHTTPMethod(t.Context(), "GET")

	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().QueryRowContext(gomock.Any(), readQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRowContext(ctx, readQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}
func TestResolver_QueryRow_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryRowContext(gomock.Any(), writeQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRow(writeQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_QueryRowContext_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	readQuery := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	ctx := WithHTTPMethod(t.Context(), "GET")

	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().QueryRowContext(gomock.Any(), readQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRowContext(ctx, readQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_QueryRowContext_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryRowContext(gomock.Any(), writeQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRowContext(t.Context(), writeQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_ExecContext_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedResult := NewMockResult(mocks.Ctrl)
	writeQuery := "UPDATE users SET name = ? WHERE id = ?"
	args := []any{"new_name", 1}

	mocks.Primary.EXPECT().ExecContext(gomock.Any(), writeQuery, args[0], args[1]).Return(expectedResult, nil)

	result, err := mocks.Resolver.ExecContext(t.Context(), writeQuery, args[0], args[1])

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestResolver_Exec_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedResult := NewMockResult(mocks.Ctrl)
	writeQuery := "UPDATE users SET name = ? WHERE id = ?"
	args := []any{"new_name", 1}

	mocks.Primary.EXPECT().ExecContext(gomock.Any(), writeQuery, args[0], args[1]).Return(expectedResult, nil)

	result, err := mocks.Resolver.Exec(writeQuery, args[0], args[1])

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestResolver_Select_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	data := &struct{ Name string }{}
	readQuery := "SELECT name FROM users WHERE id = ?"
	args := []any{1}

	ctx := WithHTTPMethod(t.Context(), "GET")

	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().Select(gomock.Any(), data, readQuery, args[0])

	mocks.Resolver.Select(ctx, data, readQuery, args[0])
}

func TestResolver_Select_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	data := &struct{ ID int64 }{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().Select(gomock.Any(), data, writeQuery, args[0])

	mocks.Resolver.Select(t.Context(), data, writeQuery, args[0])
}

func TestResolver_Prepare_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedStmt := &sql.Stmt{}
	query := "SELECT * FROM users WHERE id = ?"

	mocks.Primary.EXPECT().Prepare(query).Return(expectedStmt, nil)

	stmt, err := mocks.Resolver.Prepare(query)

	require.NoError(t, err)
	assert.Equal(t, expectedStmt, stmt)
}

func TestResolver_Begin_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedTx := &gofrSQL.Tx{}

	mocks.Primary.EXPECT().Begin().Return(expectedTx, nil)

	tx, err := mocks.Resolver.Begin()

	require.NoError(t, err)
	assert.Equal(t, expectedTx, tx)
}

func TestResolver_Dialect(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedDialect := "mysql"
	mocks.Primary.EXPECT().Dialect().Return(expectedDialect)

	dialect := mocks.Resolver.Dialect()
	assert.Equal(t, expectedDialect, dialect)
}

func TestResolver_Close(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	// Test primary close
	mocks.Primary.EXPECT().Close().Return(nil)

	// Test replicas close
	for _, replica := range mocks.MockReplicas {
		replica.EXPECT().Close().Return(nil)
	}

	err := mocks.Resolver.Close()
	assert.NoError(t, err)
}

func TestResolver_Close_WithError(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	// Primary returns error
	mocks.Primary.EXPECT().Close().Return(errTestReplicaFailed)

	// Replicas don't error
	for _, replica := range mocks.MockReplicas {
		replica.EXPECT().Close().Return(nil)
	}

	err := mocks.Resolver.Close()
	assert.Equal(t, errTestReplicaFailed, err)
}

func TestResolver_QueryContext_WithFallback(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	ctx := WithHTTPMethod(t.Context(), "GET")

	// First replica attempt fails
	mocks.Strategy.EXPECT().Next(2).Return(0)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(nil, errTestReplicaFailed)

	mocks.Logger.EXPECT().Warn("Falling back to primary for read operation")

	// Fallback to primary succeeds
	mocks.Primary.EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(ctx, readQuery)

	require.NoError(t, rows.Err())
	require.NoError(t, err)
	assert.NotNil(t, rows)
}

func TestResolver_HealthCheck(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	primaryHealth := &datasource.Health{
		Status: healthStatusUP,
		Details: map[string]any{
			"connections": 5,
		},
	}

	replica1Health := &datasource.Health{
		Status: healthStatusUP,
		Details: map[string]any{
			"connections": 3,
		},
	}

	replica2Health := &datasource.Health{
		Status: healthStatusDOWN,
		Details: map[string]any{
			"error": "connection refused",
		},
	}

	mocks.Primary.EXPECT().HealthCheck().Return(primaryHealth)
	mocks.MockReplicas[0].EXPECT().HealthCheck().Return(replica1Health)
	mocks.MockReplicas[1].EXPECT().HealthCheck().Return(replica2Health)

	health := mocks.Resolver.HealthCheck()

	assert.Equal(t, healthStatusUP, health.Status)
	assert.Equal(t, primaryHealth, health.Details["primary"])

	replicas := health.Details["replicas"].([]any)
	assert.Len(t, replicas, 2)

	stats := health.Details["stats"].(map[string]any)
	assert.Contains(t, stats, "primaryReads")
	assert.Contains(t, stats, "replicaReads")
	assert.Contains(t, stats, "totalQueries")
}

func TestResolver_AddTrace(t *testing.T) {
	mocks := setupMocks(t)
	defer mocks.Ctrl.Finish()

	method := "query"
	query := "SELECT * FROM users"

	_, span := mocks.Resolver.addTrace(t.Context(), method, query)

	assert.NotNil(t, span)
}

// setupNewResolverTest creates a test setup for NewResolver tests.
func setupNewResolverTest(t *testing.T, replicaCount int, expectLogger bool) (*gomock.Controller, *MockDB,
	[]container.DB, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockPrimary := NewMockDB(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Create replicas
	replicas := make([]container.DB, replicaCount)
	for i := 0; i < replicaCount; i++ {
		replicas[i] = NewMockDB(ctrl)
	}

	// Set up common expectations
	mockMetrics.EXPECT().NewHistogram(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	mockMetrics.EXPECT().NewGauge(gomock.Any(), gomock.Any()).Times(5)

	if expectLogger {
		mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).Times(1)
	}

	return ctrl, mockPrimary, replicas, mockLogger, mockMetrics
}

func TestNewResolver_WithOptions(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	customStrategy := NewMockStrategy(ctrl)
	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithStrategy(customStrategy),
		WithFallback(false),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	assert.Equal(t, customStrategy, dbResolver.strategy)
	assert.False(t, dbResolver.readFallback)
	assert.Len(t, dbResolver.replicas, 1)
}

func TestNewResolver_WithPrimaryRoutes(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{
		"/api/v1/admin":  true,
		"/api/v2/write*": true,
		"/health":        true,
	}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	assert.Len(t, dbResolver.primaryRoutes, 3)
	assert.Len(t, dbResolver.primaryPrefixes, 1)
	assert.Contains(t, dbResolver.primaryPrefixes, "/api/v2/write")
}

func TestNewResolver_NoReplicas(t *testing.T) {
	ctrl, mockPrimary, _, mockLogger, mockMetrics := setupNewResolverTest(t, 0, true)
	defer ctrl.Finish()

	resolver := NewResolver(mockPrimary, nil, mockLogger, mockMetrics)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	assert.Empty(t, dbResolver.replicas)
	assert.Nil(t, dbResolver.strategy)
}

func TestNewResolver_NoLogger(t *testing.T) {
	ctrl, mockPrimary, replicas, _, mockMetrics := setupNewResolverTest(t, 1, false)
	defer ctrl.Finish()

	resolver := NewResolver(mockPrimary, replicas, nil, mockMetrics)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	assert.Nil(t, dbResolver.logger)
}

func TestNewResolver_NoMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrimary := NewMockDB(ctrl)
	mockReplica := NewMockDB(ctrl)
	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).Times(1)

	resolver := NewResolver(mockPrimary, []container.DB{mockReplica}, mockLogger, nil)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	assert.Nil(t, dbResolver.metrics)
}

func TestResolver_QueryContext_NoReplicasConfigured(t *testing.T) {
	ctrl, mockPrimary, _, mockLogger, mockMetrics := setupNewResolverTest(t, 0, true)
	defer ctrl.Finish()

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dbresolver_query_duration", gomock.Any(), gomock.Any()).Times(1)

	resolver := NewResolver(mockPrimary, nil, mockLogger, mockMetrics)

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"
	ctx := WithHTTPMethod(t.Context(), "GET")

	mockPrimary.EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := resolver.QueryContext(ctx, readQuery)

	require.NoError(t, rows.Err())
	require.NoError(t, err)
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_ShouldUseReplica_WithRequestPath(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{"/admin": true}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	ctx := WithHTTPMethod(t.Context(), "GET")
	ctx = WithRequestPath(ctx, "/admin")

	useReplica := dbResolver.shouldUseReplica(ctx)
	assert.False(t, useReplica)
}

func TestResolver_ShouldUseReplica_WithPrefixMatch(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{"/api/admin/*": true}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	ctx := WithHTTPMethod(t.Context(), "GET")
	ctx = WithRequestPath(ctx, "/api/admin/users")

	useReplica := dbResolver.shouldUseReplica(ctx)
	assert.False(t, useReplica)
}

func TestResolver_ShouldUseReplica_NoPathMatch(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{"/admin": true}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	ctx := WithHTTPMethod(t.Context(), "GET")
	ctx = WithRequestPath(ctx, "/api/users")

	useReplica := dbResolver.shouldUseReplica(ctx)
	assert.True(t, useReplica)
}

func TestResolver_ShouldUseReplica_NoMethod(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	useReplica := dbResolver.shouldUseReplica(t.Context())
	assert.False(t, useReplica)
}

func TestResolver_IsPrimaryRoute_ExactMatch(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{"/exact/path": true}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	isPrimary := dbResolver.isPrimaryRoute("/exact/path")
	assert.True(t, isPrimary)
}

func TestResolver_IsPrimaryRoute_NoMatch(t *testing.T) {
	ctrl, mockPrimary, replicas, mockLogger, mockMetrics := setupNewResolverTest(t, 1, true)
	defer ctrl.Finish()

	routes := map[string]bool{"/admin": true}

	resolver := NewResolver(mockPrimary, replicas, mockLogger, mockMetrics,
		WithPrimaryRoutes(routes),
	)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	isPrimary := dbResolver.isPrimaryRoute("/api/users")
	assert.False(t, isPrimary)
}

func TestResolver_UpdateMetrics_NoMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrimary := NewMockDB(ctrl)
	mockReplica := NewMockDB(ctrl)
	mockLogger := NewMockLogger(ctrl)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).Times(1)

	resolver := NewResolver(mockPrimary, []container.DB{mockReplica}, mockLogger, nil)

	dbResolver, ok := resolver.(*Resolver)
	require.True(t, ok)

	dbResolver.updateMetrics()

	assert.Nil(t, dbResolver.metrics)
}

func TestQueryLog_PrettyPrint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	tests := []struct {
		name  string
		log   *QueryLog
		check func(string)
	}{
		{
			name: "basic formatting",
			log: &QueryLog{
				Type:     "SQL",
				Query:    "SELECT * FROM users WHERE id = 1",
				Duration: 1500,
				Target:   "replica-1",
				IsRead:   true,
			},
			check: func(output string) {
				assert.Contains(t, output, "DBRESOLVER")
				assert.Contains(t, output, "1500")
				assert.Contains(t, output, "replica-1")
			},
		},
		{
			name: "multiple whitespace cleanup",
			log: &QueryLog{
				Type:     "SQL",
				Query:    "SELECT  *   FROM   users   WHERE  id = 1",
				Duration: 2000,
				Target:   "primary",
				IsRead:   false,
			},
			check: func(output string) {
				assert.Contains(t, output, "SELECT * FROM users WHERE id = 1")
				assert.NotContains(t, output, "SELECT  *")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			var capturedLog string

			mockLogger.EXPECT().Debug(gomock.Any()).Do(func(args ...any) {
				capturedLog = fmt.Sprint(args...)
			}).Times(1)

			tt.log.PrettyPrint(mockLogger)

			tt.check(capturedLog)
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"multiple spaces", "SELECT  *   FROM   users", "SELECT * FROM users"},
		{"leading/trailing whitespace", "  SELECT * FROM users  ", "SELECT * FROM users"},
		{"tabs", "SELECT\t*\tFROM\tusers", "SELECT * FROM users"},
		{"newlines", "SELECT *\nFROM users\nWHERE id = 1", "SELECT * FROM users WHERE id = 1"},
		{"mixed whitespace", "  SELECT  \t*\n  FROM   users\t\n", "SELECT * FROM users"},
		{"empty string", "", ""},
		{"only whitespace", "   \t\n  ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clean(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
