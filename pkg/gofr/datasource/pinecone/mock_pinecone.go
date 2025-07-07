package pinecone

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockPineconeClient is a mock implementation of the Pinecone interface.
type MockPineconeClient struct {
	mock.Mock
}

func (m *MockPineconeClient) Connect() {
	m.Called()
}

func (m *MockPineconeClient) HealthCheck(ctx context.Context) (any, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockPineconeClient) UseLogger(logger any) {
	m.Called(logger)
}

func (m *MockPineconeClient) UseMetrics(metrics any) {
	m.Called(metrics)
}

func (m *MockPineconeClient) UseTracer(tracer any) {
	m.Called(tracer)
}

func (m *MockPineconeClient) ListIndexes(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPineconeClient) DescribeIndex(ctx context.Context, indexName string) (map[string]any, error) {
	args := m.Called(ctx, indexName)
	return args.Get(0).(map[string]any), args.Error(1)
}
func (m *MockPineconeClient) CreateIndex(
	ctx context.Context,
	indexName string,
	dimension int,
	metric string,
	options map[string]any,
) error {
	args := m.Called(ctx, indexName, dimension, metric, options)
	return args.Error(0)
}

func (m *MockPineconeClient) DeleteIndex(ctx context.Context, indexName string) error {
	args := m.Called(ctx, indexName)
	return args.Error(0)
}

func (m *MockPineconeClient) Upsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error) {
	args := m.Called(ctx, indexName, namespace, vectors)
	return args.Int(0), args.Error(1)
}

func (m *MockPineconeClient) Query(ctx context.Context, params *QueryParams) ([]any, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]any), args.Error(1)
}
func (m *MockPineconeClient) Fetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error) {
	args := m.Called(ctx, indexName, namespace, ids)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *MockPineconeClient) Delete(ctx context.Context, indexName, namespace string, ids []string) error {
	args := m.Called(ctx, indexName, namespace, ids)
	return args.Error(0)
}

// MockLogger for testing.
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(args ...any) {
	m.Called(args...)
}

func (m *MockLogger) Debugf(format string, args ...any) {
	m.Called(format, args)
}

func (m *MockLogger) Info(args ...any) {
	m.Called(args...)
}

func (m *MockLogger) Infof(format string, args ...any) {
	m.Called(format, args)
}

func (m *MockLogger) Warn(args ...any) {
	m.Called(args...)
}

func (m *MockLogger) Warnf(format string, args ...any) {
	m.Called(format, args)
}

func (m *MockLogger) Error(args ...any) {
	m.Called(args...)
}

func (m *MockLogger) Errorf(format string, args ...any) {
	m.Called(format, args)
}

func (m *MockLogger) Fatal(args ...any) {
	m.Called(args...)
}

func (m *MockLogger) Fatalf(format string, args ...any) {
	m.Called(format, args)
}

func (m *MockLogger) Log(level any, args ...any) {
	m.Called(level, args)
}

func (m *MockLogger) Logf(level any, format string, args ...any) {
	m.Called(level, format, args)
}
