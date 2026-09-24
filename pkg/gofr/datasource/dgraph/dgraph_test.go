package dgraph

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	"go.opentelemetry.io/otel/trace/noop"
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
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	client.tracer = otel.GetTracerProvider().Tracer("gofr-dgraph")

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mockTxn.EXPECT().Query(gomock.Any(), "my query").Return(nil, errQueryFailed)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_query_duration", gomock.Any())

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
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	query := "my query with vars"
	vars := map[string]string{"$var": "value"}

	mockTxn.EXPECT().QueryWithVars(gomock.Any(), query, vars).Return(nil, errQueryFailed)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_query_with_vars_duration", gomock.Any())

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
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	mockTxn := NewMockTxn(mockDgraphClient.ctrl)
	mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)

	mutation := &api.Mutation{CommitNow: true}

	mockTxn.EXPECT().Mutate(gomock.Any(), mutation).Return(nil, errMutationFailed)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_mutate_duration", gomock.Any())

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
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	op := &api.Operation{}
	mockDgraphClient.EXPECT().Alter(gomock.Any(), op).Return(errAlterFailed)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

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
	client, mockDgraphClient, mockLogger, mockMetrics := setupDB(t)

	fieldName := "email"
	expectedOp := &api.Operation{DropAttr: fieldName}

	mockDgraphClient.EXPECT().Alter(gomock.Any(), expectedOp).Return(errAlterFailed)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "dgraph_alter_duration", gomock.Any())

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
		desc      string
		resp      *api.Response
		queryErr  error
		expLog    func(m *MockLogger)
		expStatus any
		expErr    error
	}{
		{desc: "healthy", resp: &api.Response{Json: []byte(`{"health":[]}`)}, expLog: func(*MockLogger) {}, expStatus: "UP"},
		{
			desc: "query error", resp: nil, queryErr: errQueryFailed,
			expLog:    func(m *MockLogger) { m.EXPECT().Error("dgraph health check failed: ", errQueryFailed) },
			expStatus: "DOWN", expErr: errHealthCheckFailed,
		},
		{
			// Only assert that a failure is logged; the log content on this path is not part of the contract.
			desc: "empty response", resp: &api.Response{},
			expLog:    func(m *MockLogger) { m.EXPECT().Error(gomock.Any()) },
			expStatus: "DOWN", expErr: errHealthCheckFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, mockDgraphClient, mockLogger, _ := setupDB(t)

			mockTxn := NewMockTxn(mockDgraphClient.ctrl)
			mockDgraphClient.EXPECT().NewTxn().Return(mockTxn)
			mockTxn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(tc.resp, tc.queryErr)
			tc.expLog(mockLogger)

			status, err := client.HealthCheck(t.Context())

			require.ErrorIs(t, err, tc.expErr)
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

// recordingTracer is a trace.Tracer that remembers every span it starts.
type recordingTracer struct {
	embedded.Tracer

	spans []*recordingSpan
}

func (r *recordingTracer) Start(ctx context.Context, name string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	s := &recordingSpan{Span: noop.Span{}, name: name}
	r.spans = append(r.spans, s)

	return trace.ContextWithSpan(ctx, s), s
}

// recordingSpan counts End calls and captures the recorded error and status.
type recordingSpan struct {
	trace.Span

	name   string
	ended  int
	errs   []error
	status codes.Code
}

func (s *recordingSpan) End(...trace.SpanEndOption) { s.ended++ }

func (s *recordingSpan) RecordError(err error, _ ...trace.EventOption) { s.errs = append(s.errs, err) }

func (s *recordingSpan) SetStatus(code codes.Code, _ string) { s.status = code }

type spanEndCase struct {
	desc      string
	setup     func(dg *MockDgraphClient, txn *MockTxn)
	call      func(ctx context.Context, c *Client) error
	expSpan   string
	expMetric string
	expHistos int
	expErr    error
	expErrs   []error
	expStatus codes.Code
}

func spanEndCases() []spanEndCase {
	mutation := &api.Mutation{CommitNow: true}
	op := &api.Operation{Schema: "name: string ."}
	vars := map[string]string{"$a": "x"}
	okResp := &api.Response{Json: []byte(`{"q":[]}`)}

	query := func(ctx context.Context, c *Client) error { _, err := c.Query(ctx, "q"); return err }
	queryWithVars := func(ctx context.Context, c *Client) error { _, err := c.QueryWithVars(ctx, "q", vars); return err }
	mutate := func(ctx context.Context, c *Client) error { _, err := c.Mutate(ctx, mutation); return err }
	alter := func(ctx context.Context, c *Client) error { return c.Alter(ctx, op) }

	return []spanEndCase{
		{
			desc: "query success",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().Query(gomock.Any(), "q").Return(okResp, nil)
			},
			call:    query,
			expSpan: "dgraph-query", expMetric: "dgraph_query_duration", expHistos: 1, expStatus: codes.Unset,
		},
		{
			desc: "query error",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().Query(gomock.Any(), "q").Return(nil, errQueryFailed)
			},
			call:    query,
			expSpan: "dgraph-query", expMetric: "dgraph_query_duration", expHistos: 1,
			expErr: errQueryFailed, expErrs: []error{errQueryFailed}, expStatus: codes.Error,
		},
		{
			desc: "query with vars success",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().QueryWithVars(gomock.Any(), "q", vars).Return(okResp, nil)
			},
			call:    queryWithVars,
			expSpan: "dgraph-query-with-vars", expMetric: "dgraph_query_with_vars_duration", expHistos: 1, expStatus: codes.Unset,
		},
		{
			desc: "query with vars error",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().QueryWithVars(gomock.Any(), "q", vars).Return(nil, errQueryFailed)
			},
			call:    queryWithVars,
			expSpan: "dgraph-query-with-vars", expMetric: "dgraph_query_with_vars_duration", expHistos: 1,
			expErr: errQueryFailed, expErrs: []error{errQueryFailed}, expStatus: codes.Error,
		},
		{
			desc: "mutate success",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().Mutate(gomock.Any(), mutation).Return(okResp, nil)
			},
			call:    mutate,
			expSpan: "dgraph-mutate", expMetric: "dgraph_mutate_duration", expHistos: 1, expStatus: codes.Unset,
		},
		{
			desc: "mutate error",
			setup: func(dg *MockDgraphClient, txn *MockTxn) {
				dg.EXPECT().NewTxn().Return(txn)
				txn.EXPECT().Mutate(gomock.Any(), mutation).Return(nil, errMutationFailed)
			},
			call:    mutate,
			expSpan: "dgraph-mutate", expMetric: "dgraph_mutate_duration", expHistos: 1,
			expErr: errMutationFailed, expErrs: []error{errMutationFailed}, expStatus: codes.Error,
		},
		{
			desc:    "mutate invalid type",
			setup:   func(*MockDgraphClient, *MockTxn) {},
			call:    func(ctx context.Context, c *Client) error { _, err := c.Mutate(ctx, "bad"); return err },
			expSpan: "dgraph-mutate", expErr: errInvalidMutation, expErrs: []error{errInvalidMutation}, expStatus: codes.Error,
		},
		{
			desc: "alter success",
			setup: func(dg *MockDgraphClient, _ *MockTxn) {
				dg.EXPECT().Alter(gomock.Any(), op).Return(nil)
			},
			call:    alter,
			expSpan: "dgraph-alter", expMetric: "dgraph_alter_duration", expHistos: 1, expStatus: codes.Unset,
		},
		{
			desc: "alter error",
			setup: func(dg *MockDgraphClient, _ *MockTxn) {
				dg.EXPECT().Alter(gomock.Any(), op).Return(errAlterFailed)
			},
			call:    alter,
			expSpan: "dgraph-alter", expMetric: "dgraph_alter_duration", expHistos: 1,
			expErr: errAlterFailed, expErrs: []error{errAlterFailed}, expStatus: codes.Error,
		},
		{
			desc:    "alter invalid type",
			setup:   func(*MockDgraphClient, *MockTxn) {},
			call:    func(ctx context.Context, c *Client) error { return c.Alter(ctx, "bad") },
			expSpan: "dgraph-alter", expErr: errInvalidOperation, expErrs: []error{errInvalidOperation}, expStatus: codes.Error,
		},
	}
}

func TestClient_SpanEndedOnEveryPath(t *testing.T) {
	for _, tc := range spanEndCases() {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)
			mockDgraph := NewMockDgraphClient(ctrl)
			tracer := &recordingTracer{}

			client := New(Config{})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)
			client.UseTracer(tracer)
			client.client = mockDgraph

			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), tc.expMetric, gomock.Any()).Times(tc.expHistos)
			tc.setup(mockDgraph, NewMockTxn(ctrl))

			err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, tc.expErr)
			require.Len(t, tracer.spans, 1)
			require.Equal(t, tc.expSpan, tracer.spans[0].name)
			require.Equal(t, 1, tracer.spans[0].ended, "span must be ended exactly once")
			require.Equal(t, tc.expErrs, tracer.spans[0].errs)
			require.Equal(t, tc.expStatus, tracer.spans[0].status)
		})
	}
}
