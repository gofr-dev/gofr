package dgraph

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
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
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-dgraph"))

	mockDgraphClient := NewMockDgraphClient(ctrl)
	client.client = mockDgraphClient

	return client, mockDgraphClient, mockLogger, mockMetrics
}

func TestClient_Connect_Failure(t *testing.T) {
	client, _, mockLogger, mockMetrics := setupDB(t)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any())

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(2)

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

func Test_ApplySchema_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	schema := "name: string @index(exact) ."
	expectedOp := &api.Operation{Schema: schema}

	mockDgraphClient.EXPECT().Alter(gomock.Any(), expectedOp).Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Debugf("dgraph alter succeeded in %dµs", gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

	err := client.ApplySchema(context.Background(), schema)

	require.NoError(t, err, "Test_ApplySchema_Success Failed!")
}

func Test_ApplySchema_EmptySchema(t *testing.T) {
	config := Config{Host: "localhost", Port: "9080"}
	client := New(config)

	err := client.ApplySchema(context.Background(), "")

	require.Equal(t, err, errEmptySchema, "Test_ApplySchema_EmptySchema Failed!")
}

func Test_AddOrUpdateField_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	fieldName := "email"
	fieldType := "string"
	directives := "@index(hash)"
	expectedSchema := "email: string @index(hash)."
	expectedOp := &api.Operation{Schema: expectedSchema}

	mockDgraphClient.EXPECT().Alter(gomock.Any(), expectedOp).Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Debugf("dgraph alter succeeded in %dµs", gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

	err := client.AddOrUpdateField(context.Background(), fieldName, fieldType, directives)

	require.NoError(t, err, "Test_AddOrUpdateField_Success Failed!")
}

func Test_AddOrUpdateField_EmptyFieldName(t *testing.T) {
	config := Config{Host: "localhost", Port: "9080"}
	client := New(config)

	err := client.AddOrUpdateField(context.Background(), "", "string", "@index(hash)")

	require.Equal(t, err, errEmptyField, "Test_AddOrUpdateField_EmptyFieldName Failed!")
}

func Test_DropField_Success(t *testing.T) {
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	fieldName := "email"
	expectedOp := &api.Operation{DropAttr: fieldName}

	mockDgraphClient.EXPECT().Alter(gomock.Any(), expectedOp).Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Debugf("dgraph alter succeeded in %dµs", gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

	err := client.DropField(context.Background(), fieldName)

	require.NoError(t, err, "Test_DropField_Success Failed!")
}

func Test_DropField_Error(t *testing.T) {
	client, mockDgraphClient, mockLogger, _ := setupDB(t)

	fieldName := "email"
	expectedOp := &api.Operation{DropAttr: fieldName}

	mockDgraphClient.EXPECT().Alter(gomock.Any(), expectedOp).Return(errAlterFailed)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Log(gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("dgraph alter failed: ", errAlterFailed)

	err := client.DropField(context.Background(), fieldName)

	require.ErrorIs(t, err, errAlterFailed, "Test_DropField_Error Failed!")
}

// fakeDgraphServer is an in-process Dgraph gRPC server whose Query answers the
// connection health check.
type fakeDgraphServer struct {
	api.UnimplementedDgraphServer
}

func (*fakeDgraphServer) Query(context.Context, *api.Request) (*api.Response, error) {
	return &api.Response{Json: []byte(`{"health":[]}`)}, nil
}

// startFakeDgraphServer serves fakeDgraphServer on a loopback listener and returns
// its host and port.
func startFakeDgraphServer(t *testing.T) (host, port string) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	api.RegisterDgraphServer(srv, &fakeDgraphServer{})

	go func() { _ = srv.Serve(lis) }()

	t.Cleanup(srv.Stop)

	host, port, err = net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)

	return host, port
}

func TestClient_Connect(t *testing.T) {
	host, port := startFakeDgraphServer(t)

	tests := []struct {
		desc       string
		config     Config
		setupMocks func(l *MockLogger, m *MockMetrics)
		expClient  bool
	}{
		{
			desc:   "invalid address fails client creation",
			config: Config{Host: "bad\x00host", Port: "9080"},
			setupMocks: func(l *MockLogger, _ *MockMetrics) {
				l.EXPECT().Debugf("connecting to Dgraph at %v", "bad\x00host:9080")
				l.EXPECT().Errorf("error while connecting to Dgraph, err: %v", gomock.Any())
			},
			expClient: false,
		},
		{
			desc:   "healthy server connects",
			config: Config{Host: host, Port: port},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				l.EXPECT().Debugf("connecting to Dgraph at %v", host+":"+port)
				m.EXPECT().NewHistogram(gomock.Any(), gomock.Any(), gomock.Any()).Times(4)
				l.EXPECT().Logf("connected to Dgraph server at %v:%v", host, port)
			},
			expClient: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			tc.setupMocks(mockLogger, mockMetrics)

			client := New(tc.config)
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			client.Connect()

			require.Equal(t, tc.expClient, client.client != nil)
		})
	}
}

func TestClient_HealthCheck(t *testing.T) {
	tests := []struct {
		desc        string
		resp        *api.Response
		queryErr    error
		expLogErr   error
		expLogs     int
		expStatus   any
		expErr      error
		expErrCause error
	}{
		{
			desc: "healthy", resp: &api.Response{Json: []byte(`{"health":[]}`)}, queryErr: nil,
			expLogErr: nil, expLogs: 0, expStatus: "UP", expErr: nil, expErrCause: nil,
		},
		{
			desc: "query error", resp: nil, queryErr: errQueryFailed,
			expLogErr: errQueryFailed, expLogs: 1, expStatus: "DOWN",
			expErr: errHealthCheckFailed, expErrCause: errQueryFailed,
		},
		{
			desc: "empty response", resp: &api.Response{}, queryErr: nil,
			expLogErr: errEmptyHealthResponse, expLogs: 1, expStatus: "DOWN",
			expErr: errHealthCheckFailed, expErrCause: errEmptyHealthResponse,
		},
		{
			desc: "nil response without error", resp: nil, queryErr: nil,
			expLogErr: errEmptyHealthResponse, expLogs: 1, expStatus: "DOWN",
			expErr: errHealthCheckFailed, expErrCause: errEmptyHealthResponse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockDgraphClient := NewMockDgraphClient(ctrl)
			mockTxn := NewMockTxn(ctrl)

			client := New(Config{Host: "localhost", Port: "9080"})
			client.UseLogger(mockLogger)
			client.client = mockDgraphClient

			mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)
			mockTxn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(tc.resp, tc.queryErr)
			mockLogger.EXPECT().Errorf("dgraph health check failed: %v", tc.expLogErr).Times(tc.expLogs)

			status, err := client.HealthCheck(t.Context())

			require.ErrorIs(t, err, tc.expErr)
			require.ErrorIs(t, err, tc.expErrCause)
			require.Equal(t, tc.expStatus, status)
		})
	}
}

func Test_mutationToString(t *testing.T) {
	tests := []struct {
		desc     string
		mutation *api.Mutation
		expected string
	}{
		{desc: "valid json is compacted", mutation: &api.Mutation{SetJson: []byte("{\n  \"name\": \"gofr\"\n}")}, expected: `{"name":"gofr"}`},
		{desc: "invalid json", mutation: &api.Mutation{SetJson: []byte("{bad")}, expected: ""},
		{desc: "no set json", mutation: &api.Mutation{}, expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, mutationToString(tc.mutation))
		})
	}
}
