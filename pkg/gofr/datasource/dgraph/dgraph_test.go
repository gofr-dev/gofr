package dgraph

import (
	"context"
	"errors"
	"testing"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errQueryFailed    = errors.New("query failed")
	errAlterFailed    = errors.New("alter failed")
	errMutationFailed = errors.New("mutation failed")
)

func setupDB(t *testing.T) (*Client, *MockDgraphClient, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := Config{Host: "localhost", Port: "9080"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	mockDgraphClient := NewMockDgraphClient(ctrl)
	client.client = mockDgraphClient

	return client, mockDgraphClient, mockLogger, mockMetrics
}

func TestClient_Connect_Failure(t *testing.T) {
	client, _, mockLogger, mockMetrics := setupDB(t)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any())
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())

	// Mock Metric behavior
	mockMetrics.EXPECT().NewHistogram(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Perform the connect operation
	client.Connect()

	require.True(t, mockLogger.ctrl.Satisfied())
	require.True(t, mockMetrics.ctrl.Satisfied())
}

func Test_Query_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mockTxn.EXPECT().Query(gomock.Any(), "my query").Return(&api.Response{Json: []byte(`{"result": "success"}`)}, nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Debugf("dgraph query succeeded in %dµs", gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_query_duration", gomock.Any())

	resp, err := client.Query(context.Background(), "my query")

	require.NoError(t, err, "Test_Query_Success Failed!")
	require.NotNil(t, resp, "Test_Query_Success Failed!")
	require.Equal(t, &api.Response{Json: []byte(`{"result": "success"}`)}, resp, "Test_Query_Success Failed!")
}

func Test_Query_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	client.tracer = otel.GetTracerProvider().Tracer("gofr-dgraph")

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mockTxn.EXPECT().Query(gomock.Any(), "my query").Return(nil, errQueryFailed)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("dgraph query failed: ", errQueryFailed)

	resp, err := client.Query(context.Background(), "my query")

	require.EqualError(t, err, errQueryFailed.Error(), "Test_Query_Error Failed!")
	require.Nil(t, resp, "Test_Query_Error Failed!")
}

func Test_QueryWithVars_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	query := "my query with vars"
	vars := map[string]string{"$var": "value"}

	mockTxn.EXPECT().QueryWithVars(gomock.Any(), query, vars).Return(&api.Response{Json: []byte(`{"result": "success"}`)}, nil)

	mockLogger.EXPECT().Debugf("dgraph queryWithVars succeeded in %dµs", gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_query_with_vars_duration", gomock.Any())

	// Call the QueryWithVars method
	resp, err := client.QueryWithVars(context.Background(), query, vars)

	require.NoError(t, err, "Test_QueryWithVars_Success Failed!")
	require.NotNil(t, resp, "Test_QueryWithVars_Success Failed!")
	require.Equal(t, &api.Response{Json: []byte(`{"result": "success"}`)}, resp, "Test_QueryWithVars_Success Failed!")
}

func Test_QueryWithVars_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	query := "my query with vars"
	vars := map[string]string{"$var": "value"}

	mockTxn.EXPECT().QueryWithVars(gomock.Any(), query, vars).Return(nil, errQueryFailed)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Error("dgraph queryWithVars failed: ", errQueryFailed)
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)

	// Call the QueryWithVars method
	resp, err := client.QueryWithVars(context.Background(), query, vars)

	require.ErrorIs(t, err, errQueryFailed, "Test_QueryWithVars_Error Failed!")
	require.Nil(t, resp, "Test_QueryWithVars_Error Failed!")
}

func Test_Mutate_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mutation := &api.Mutation{CommitNow: true}

	mockTxn.EXPECT().Mutate(gomock.Any(), mutation).Return(&api.Response{Json: []byte(`{"result": "mutation success"}`)}, nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Debugf("dgraph mutation succeeded in %dµs", gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_mutate_duration", gomock.Any())

	// Call the Mutate method
	resp, err := client.Mutate(context.Background(), mutation)

	require.NoError(t, err, "Test_Mutate_Success Failed!")
	require.NotNil(t, resp, "Test_Mutate_Success Failed!")
	require.Equal(t, &api.Response{Json: []byte(`{"result": "mutation success"}`)}, resp, "Test_Mutate_Success Failed!")
}

func Test_Mutate_InvalidMutation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := Config{Host: "localhost", Port: "9080"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	mockDgraphClient := NewMockDgraphClient(ctrl)
	client.client = mockDgraphClient

	// Call the Mutate method with an invalid type
	resp, err := client.Mutate(context.Background(), "invalid mutation")

	require.EqualError(t, err, errInvalidMutation.Error(), "Test_Mutate_InvalidMutation Failed!")
	require.Nil(t, resp, "Test_Mutate_InvalidMutation Failed!")
}

func Test_Mutate_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mutation := &api.Mutation{CommitNow: true}

	mockTxn.EXPECT().Mutate(gomock.Any(), mutation).Return(nil, errMutationFailed)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Error("dgraph mutation failed: ", errMutationFailed)
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)

	// Call the Mutate method
	resp, err := client.Mutate(context.Background(), mutation)

	require.EqualError(t, err, "mutation failed", "Test_Mutate_Error Failed!")
	require.Nil(t, resp, "Test_Mutate_Error Failed!")
}

func Test_Alter_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	op := &api.Operation{}
	mockDgraphClient.EXPECT().Alter(gomock.Any(), op).Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Debugf("dgraph alter succeeded in %dµs", gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

	err := client.Alter(context.Background(), op)

	require.NoError(t, err, "Test_Alter_Success Failed!")
}

func Test_Alter_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	op := &api.Operation{}
	mockDgraphClient.EXPECT().Alter(gomock.Any(), op).Return(errAlterFailed)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("dgraph alter failed: ", errAlterFailed)

	err := client.Alter(context.Background(), op)

	require.ErrorIs(t, err, errAlterFailed, "Test_Alter_Error Failed!")
}

func Test_Alter_InvalidOperation(t *testing.T) {
	client, _, mockLogger, _ := setupDB(t)

	op := "invalid operation"

	mockLogger.EXPECT().Error("invalid operation type provided to alter")

	err := client.Alter(context.Background(), op)

	require.EqualError(t, err, errInvalidOperation.Error(), "Test_Alter_InvalidOperation Failed!")
}

func Test_NewTxn(t *testing.T) {
	client, mockDgraphClient, _, _ := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	txn := client.NewTxn()

	require.NotNil(t, txn, "Test_NewTxn Failed!")
}

func Test_NewReadOnlyTxn(t *testing.T) {
	client, mockDgraphClient, _, _ := setupDB(t)

	mockReadOnlyTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewReadOnlyTxn().Return(mockReadOnlyTxn)

	txn := client.NewReadOnlyTxn()

	require.NotNil(t, txn, "Test_NewReadOnlyTxn Failed!")
}

func Test_HealthCheck_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mockLogger.EXPECT().Error("dgraph health check failed: ", errQueryFailed)

	mockQueryResponse := &api.Response{}
	mockTxn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(mockQueryResponse, errQueryFailed)

	_, err := client.HealthCheck(context.Background())

	require.EqualError(t, err, errHealthCheckFailed.Error(), "Test_HealthCheck_Error Failed!")
}
