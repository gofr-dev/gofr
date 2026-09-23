package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	surrealhttp "github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

// Test_NewClient verifies that a new client is created with the provided configuration.
func Test_NewClient(t *testing.T) {
	config := &Config{
		Host:       "localhost",
		Port:       8000,
		Username:   "root",
		Password:   "root",
		Namespace:  "test_namespace",
		Database:   "test_database",
		TLSEnabled: false,
	}

	client := New(config)
	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.Nil(t, client.db)
}

// Test_UseLogger verifies that a custom logger can be set.
func Test_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{})
	mockLogger := NewMockLogger(ctrl)

	client.UseLogger(mockLogger)
	assert.NotNil(t, client.logger)
}

// Test_UseMetrics verifies that custom metrics can be set.
func Test_UseMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{})
	mockMetrics := NewMockMetrics(ctrl)

	client.UseMetrics(mockMetrics)
	assert.NotNil(t, client.metrics)
}

// Test_UseTracer verifies that a custom tracer can be set.
func Test_UseTracer(t *testing.T) {
	client := New(&Config{})

	t.Run("valid tracer", func(t *testing.T) {
		tracer := otel.GetTracerProvider().Tracer("test-tracer")
		client.UseTracer(tracer)
		assert.NotNil(t, client.tracer)
	})

	t.Run("invalid tracer type", func(t *testing.T) {
		originalTracer := client.tracer
		client.UseTracer("invalid")
		assert.Equal(t, originalTracer, client.tracer)
	})
}

// Test_extractRecord verifies the extraction and conversion of record data.
func Test_extractRecord(t *testing.T) {
	client := &Client{
		logger: NewMockLogger(gomock.NewController(t)),
	}

	t.Run("extract map[string]any record", func(t *testing.T) {
		record := map[string]any{
			"id":    "user:1",
			"name":  "John",
			"age":   float64(30),
			"email": "john@example.com",
		}

		result, err := client.extractRecord(record)
		require.NoError(t, err)
		assert.Equal(t, "user:1", result["id"])
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, 30, result["age"])
		assert.Equal(t, "john@example.com", result["email"])
	})

	t.Run("extract map[any]any record", func(t *testing.T) {
		record := map[any]any{
			"id":    "user:2",
			"name":  "Jane",
			"age":   float64(28),
			"email": "jane@example.com",
		}

		result, err := client.extractRecord(record)
		require.NoError(t, err)
		assert.Equal(t, "user:2", result["id"])
		assert.Equal(t, "Jane", result["name"])
		assert.Equal(t, 28, result["age"])
		assert.Equal(t, "jane@example.com", result["email"])
	})

	t.Run("extract with numeric conversions", func(t *testing.T) {
		record := map[string]any{
			"float64_val": float64(42),
			"uint64_val":  uint64(99),
			"int64_val":   int64(77),
			"string_val":  "test",
		}

		result, err := client.extractRecord(record)
		require.NoError(t, err)
		assert.Equal(t, 42, result["float64_val"])
		assert.Equal(t, 99, result["uint64_val"])
		assert.Equal(t, 77, result["int64_val"])
		assert.Equal(t, "test", result["string_val"])
	})

	t.Run("extract invalid record type", func(t *testing.T) {
		record := "invalid record"
		_, err := client.extractRecord(record)
		require.ErrorIs(t, err, errUnexpectedResult)
	})

	t.Run("extract nil record", func(t *testing.T) {
		_, err := client.extractRecord(nil)
		require.ErrorIs(t, err, errUnexpectedResult)
	})
}

// Test_handleResultRecord verifies processing of different result types.
func Test_handleResultRecord(t *testing.T) {
	client := &Client{
		logger: NewMockLogger(gomock.NewController(t)),
	}

	t.Run("handle array of records", func(t *testing.T) {
		result := []any{
			map[string]any{"id": "1", "name": "Alice"},
			map[string]any{"id": "2", "name": "Bob"},
		}

		var resp []any
		client.handleResultRecord(result, &resp)

		require.Len(t, resp, 2)
		assert.Equal(t, "Alice", resp[0].(map[string]any)["name"])
		assert.Equal(t, "Bob", resp[1].(map[string]any)["name"])
	})

	t.Run("handle single record as map[string]any", func(t *testing.T) {
		result := map[string]any{"id": "user:1", "name": "Charlie"}

		var resp []any
		client.handleResultRecord(result, &resp)

		require.Len(t, resp, 1)
		extracted := resp[0].(map[string]any)
		assert.Equal(t, "Charlie", extracted["name"])
	})

	t.Run("handle single record as map[any]any", func(t *testing.T) {
		result := map[any]any{"id": "user:2", "name": "Diana"}

		var resp []any
		client.handleResultRecord(result, &resp)

		require.Len(t, resp, 1)
		extracted := resp[0].(map[string]any)
		assert.Equal(t, "Diana", extracted["name"])
	})

	t.Run("handle scalar value", func(t *testing.T) {
		result := "some scalar"

		var resp []any
		client.handleResultRecord(result, &resp)

		require.Len(t, resp, 1)
		assert.Equal(t, "some scalar", resp[0])
	})

	t.Run("handle boolean value", func(t *testing.T) {
		result := true

		var resp []any
		client.handleResultRecord(result, &resp)

		require.Len(t, resp, 1)
		assert.Equal(t, true, resp[0])
	})
}

// Test_convertValue verifies numeric type conversions.
func Test_convertValue(t *testing.T) {
	client := &Client{}

	t.Run("float64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    float64
			expected any
		}{
			{"valid float64", 42.0, 42},
			{"too large float64", math.MaxFloat64, nil},
			{"too small float64", -math.MaxFloat64, nil},
			{"zero", 0.0, 0},
			{"negative", -5.0, -5},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("uint64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    uint64
			expected any
		}{
			{"valid uint64", uint64(42), 42},
			{"too large uint64", uint64(math.MaxInt + 1), nil},
			{"zero", uint64(0), 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("int64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    int64
			expected any
		}{
			{"valid int64", int64(42), 42},
			{"max boundary", int64(math.MaxInt), int(math.MaxInt)},
			{"min boundary", int64(math.MinInt), int(math.MinInt)},
			{"zero", int64(0), 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("string conversion", func(t *testing.T) {
		input := "test string"
		result := client.convertValue(input)
		assert.Equal(t, input, result)
	})

	t.Run("default case", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := client.convertValue(input)
		assert.Equal(t, input, result)
	})

	t.Run("boolean value", func(t *testing.T) {
		input := true
		result := client.convertValue(input)
		assert.Equal(t, input, result)
	})
}

// Test_NotConnectedError verifies behavior when database is not connected.
func Test_NotConnectedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{})
	client.UseLogger(NewMockLogger(ctrl))
	client.UseMetrics(NewMockMetrics(ctrl))

	t.Run("query without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Query(ctx, "SELECT * FROM users", nil)

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})

	t.Run("select without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Select(ctx, "users")

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})

	t.Run("create without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Create(ctx, "users", map[string]any{"name": "test"})

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})

	t.Run("update without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Update(ctx, "users", "1", map[string]any{"name": "updated"})

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})

	t.Run("insert without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Insert(ctx, "users", []map[string]any{{"name": "test"}})

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})

	t.Run("delete without connection", func(t *testing.T) {
		ctx := context.Background()
		result, err := client.Delete(ctx, "users", "1")

		require.ErrorIs(t, err, errNotConnected)
		assert.Nil(t, result)
	})
}

// Test_UseDBInterface verifies that the client can use the DB interface.
func Test_UseDBInterface(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})

	mockDB := NewMockDB(ctrl)
	client.db = mockDB

	t.Run("db interface is used correctly", func(t *testing.T) {
		assert.NotNil(t, client.db)
		assert.Equal(t, mockDB, client.db)
	})
}

// Test_ExtractRecordWithNonStringKey verifies handling of non-string keys in map[any]any.
func Test_ExtractRecordWithNonStringKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).Times(1)

	client := &Client{logger: mockLogger}

	record := map[any]any{
		"id":   "user:1",
		123:    "numeric key",
		"name": "John",
	}

	result, err := client.extractRecord(record)
	require.NoError(t, err)
	// Should successfully extract string keys and skip non-string keys
	assert.Equal(t, "user:1", result["id"])
	assert.Equal(t, "John", result["name"])
	assert.NotContains(t, result, 123)
}

var (
	errInfo = errors.New("info failed")
	errUse  = errors.New("use failed")
	errAuth = errors.New("invalid credentials")
)

const (
	testNamespace = "test_ns"
	testDatabase  = "test_db"
)

// rpcErrorReply makes the fake SurrealDB server answer an RPC method with a JSON-RPC error.
type rpcErrorReply struct {
	message string
}

type testMocks struct {
	logger  *MockLogger
	metrics *MockMetrics
	db      *MockDB
}

func newTestMocks(t *testing.T) *testMocks {
	t.Helper()

	ctrl := gomock.NewController(t)

	return &testMocks{
		logger:  NewMockLogger(ctrl),
		metrics: NewMockMetrics(ctrl),
		db:      NewMockDB(ctrl),
	}
}

// expectOperationStats sets the expectations for a single sendOperationStats call.
func expectOperationStats(m *testMocks, operation string, openConnections float64) {
	m.logger.EXPECT().Debug(gomock.Any())
	m.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(),
		"namespace", testNamespace, "database", testDatabase, "operation", operation)
	m.metrics.EXPECT().SetGauge("app_surrealdb_open_connections", openConnections)
}

// surrealRPCHandler returns an in-process fake of the SurrealDB HTTP RPC endpoint.
// Each RPC method is answered with the configured reply; a nil reply yields a response without a result.
func surrealRPCHandler(t *testing.T, replies map[string]any) http.Handler {
	t.Helper()

	codec := surrealcbor.New()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var req map[string]any
		assert.NoError(t, codec.Unmarshal(body, &req))

		method, _ := req["method"].(string)
		reply, ok := replies[method]
		assert.True(t, ok, "unexpected RPC method %q", method)

		resp := map[string]any{"id": req["id"]}

		switch v := reply.(type) {
		case rpcErrorReply:
			resp["error"] = map[string]any{"code": -32000, "message": v.message}
		case nil:
		default:
			resp["result"] = v
		}

		data, err := codec.Marshal(resp)
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/cbor")
		_, _ = w.Write(data)
	})
}

// newTestDBWrapper connects a real SurrealDB HTTP client to the in-process fake server.
func newTestDBWrapper(t *testing.T, replies map[string]any) *DBWrapper {
	t.Helper()

	srv := httptest.NewServer(surrealRPCHandler(t, replies))
	t.Cleanup(srv.Close)

	u, err := url.ParseRequestURI(srv.URL)
	require.NoError(t, err)

	db, err := surrealdb.FromConnection(t.Context(), surrealhttp.New(connection.NewConfig(u)))
	require.NoError(t, err)
	require.NoError(t, db.Use(t.Context(), testNamespace, testDatabase))

	return NewDBWrapper(db)
}

func queryReply(status string, result any) []any {
	return []any{map[string]any{"status": status, "time": "1ms", "result": result}}
}

func newTestClient(m *testMocks, db DB) *Client {
	return &Client{
		config:  &Config{Host: "localhost", Port: 8000, Namespace: testNamespace, Database: testDatabase},
		db:      db,
		logger:  m.logger,
		metrics: m.metrics,
		tracer:  noop.NewTracerProvider().Tracer("test"),
	}
}

func Test_Query(t *testing.T) {
	tests := []struct {
		desc       string
		query      string
		vars       map[string]any
		replies    map[string]any
		setupMocks func(m *testMocks)
		expResult  []any
		expErrMsg  string
	}{
		{
			desc:  "success: records returned for type::thing query",
			query: "SELECT * FROM type::thing('users', $id)",
			vars:  map[string]any{"id": "1"},
			replies: map[string]any{
				"query": queryReply(statusOK, []any{map[string]any{"id": "1", "name": "Alice", "age": 30}}),
			},
			setupMocks: func(m *testMocks) { expectOperationStats(m, "query", 1) },
			expResult:  []any{map[string]any{"id": "1", "name": "Alice", "age": 30}},
		},
		{
			desc:       "error: empty result set",
			query:      "SELECT * FROM users",
			replies:    map[string]any{"query": []any{}},
			setupMocks: func(m *testMocks) { expectOperationStats(m, "query", 1) },
			expErrMsg:  errNoResult.Error(),
		},
		{
			desc:       "error: rpc failure",
			query:      "SELECT * FROM users",
			replies:    map[string]any{"query": rpcErrorReply{message: "rpc exploded"}},
			setupMocks: func(m *testMocks) { expectOperationStats(m, "query", 1) },
			expErrMsg:  "query failed: rpc exploded",
		},
		{
			desc:       "error: statement failed",
			query:      "SELECT * FROM users",
			replies:    map[string]any{"query": queryReply("ERR", "syntax error")},
			setupMocks: func(m *testMocks) { expectOperationStats(m, "query", 1) },
			expErrMsg:  "query failed: syntax error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			tc.setupMocks(m)

			client := newTestClient(m, newTestDBWrapper(t, tc.replies))

			result, err := client.Query(t.Context(), tc.query, tc.vars)

			assert.Equal(t, tc.expResult, result)
			assert.Equal(t, tc.expErrMsg, errMsg(err))
		})
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

type crudTestCase struct {
	desc      string
	operation string
	replies   map[string]any
	run       func(ctx context.Context, c *Client) (any, error)
	expResult any
	expErrMsg string
}

func testRecord() map[string]any { return map[string]any{"id": "users:1", "name": "Alice"} }

func runCRUDTests(t *testing.T, tests []crudTestCase) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			expectOperationStats(m, tc.operation, 1)

			client := newTestClient(m, newTestDBWrapper(t, tc.replies))

			result, err := tc.run(t.Context(), client)

			assert.Equal(t, tc.expResult, result)
			assert.Equal(t, tc.expErrMsg, errMsg(err))
		})
	}
}

func Test_Select(t *testing.T) {
	runCRUDTests(t, []crudTestCase{
		{
			desc:      "select: records returned",
			operation: "select",
			replies:   map[string]any{"select": []any{testRecord()}},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Select(ctx, "users") },
			expResult: []map[string]any{testRecord()},
		},
		{
			desc:      "select: no result yields empty slice",
			operation: "select",
			replies:   map[string]any{"select": nil},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Select(ctx, "users") },
			expResult: []map[string]any{},
		},
		{
			desc:      "select: rpc failure",
			operation: "select",
			replies:   map[string]any{"select": rpcErrorReply{message: "boom"}},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Select(ctx, "users") },
			expResult: []map[string]any(nil),
			expErrMsg: "select operation failed: boom",
		},
	})
}

func Test_Create(t *testing.T) {
	runCRUDTests(t, []crudTestCase{
		{
			desc:      "create: record created",
			operation: "create",
			replies:   map[string]any{"create": testRecord()},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Create(ctx, "users", map[string]any{"name": "Alice"})
			},
			expResult: testRecord(),
		},
		{
			desc:      "create: no record returned",
			operation: "create",
			replies:   map[string]any{"create": nil},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Create(ctx, "users", map[string]any{"name": "Alice"})
			},
			expResult: map[string]any(nil),
			expErrMsg: errNoRecord.Error(),
		},
		{
			desc:      "create: rpc failure",
			operation: "create",
			replies:   map[string]any{"create": rpcErrorReply{message: "boom"}},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Create(ctx, "users", map[string]any{"name": "Alice"})
			},
			expResult: map[string]any(nil),
			expErrMsg: "create operation failed: boom",
		},
	})
}

func Test_Update(t *testing.T) {
	runCRUDTests(t, []crudTestCase{
		{
			desc:      "update: record updated",
			operation: "update",
			replies:   map[string]any{"update": testRecord()},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Update(ctx, "users", "1", map[string]any{"name": "Alice"})
			},
			expResult: testRecord(),
		},
		{
			desc:      "update: no record returned",
			operation: "update",
			replies:   map[string]any{"update": nil},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Update(ctx, "users", "1", map[string]any{"name": "Alice"})
			},
			expErrMsg: errNoRecord.Error(),
		},
		{
			desc:      "update: rpc failure",
			operation: "update",
			replies:   map[string]any{"update": rpcErrorReply{message: "boom"}},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Update(ctx, "users", "1", map[string]any{"name": "Alice"})
			},
			expErrMsg: "update operation failed: boom",
		},
	})
}

func Test_Insert(t *testing.T) {
	runCRUDTests(t, []crudTestCase{
		{
			desc:      "insert: records inserted",
			operation: "insert",
			replies:   map[string]any{"insert": []any{testRecord()}},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Insert(ctx, "users", []map[string]any{{"name": "Alice"}})
			},
			expResult: []map[string]any{testRecord()},
		},
		{
			desc:      "insert: no result yields empty slice",
			operation: "insert",
			replies:   map[string]any{"insert": nil},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Insert(ctx, "users", []map[string]any{{"name": "Alice"}})
			},
			expResult: []map[string]any{},
		},
		{
			desc:      "insert: rpc failure",
			operation: "insert",
			replies:   map[string]any{"insert": rpcErrorReply{message: "boom"}},
			run: func(ctx context.Context, c *Client) (any, error) {
				return c.Insert(ctx, "users", []map[string]any{{"name": "Alice"}})
			},
			expResult: []map[string]any(nil),
			expErrMsg: "insert operation failed: boom",
		},
	})
}

func Test_Delete(t *testing.T) {
	runCRUDTests(t, []crudTestCase{
		{
			desc:      "delete: deleted record returned",
			operation: "delete",
			replies:   map[string]any{"delete": testRecord()},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Delete(ctx, "users", "1") },
			expResult: testRecord(),
		},
		{
			desc:      "delete: no record returned",
			operation: "delete",
			replies:   map[string]any{"delete": nil},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Delete(ctx, "users", "1") },
		},
		{
			desc:      "delete: rpc failure",
			operation: "delete",
			replies:   map[string]any{"delete": rpcErrorReply{message: "boom"}},
			run:       func(ctx context.Context, c *Client) (any, error) { return c.Delete(ctx, "users", "1") },
			expErrMsg: "delete operation failed: boom",
		},
	})
}

func Test_NamespaceAndDatabaseOperations(t *testing.T) {
	tests := []struct {
		desc      string
		setup     func(t *testing.T, m *testMocks) DB
		run       func(ctx context.Context, c *Client) error
		expErrMsg string
	}{
		{
			desc: "create namespace",
			setup: func(t *testing.T, m *testMocks) DB {
				t.Helper()
				expectOperationStats(m, "query", 1)
				expectOperationStats(m, "creating", 1)

				return newTestDBWrapper(t, map[string]any{"query": queryReply(statusOK, nil)})
			},
			run: func(ctx context.Context, c *Client) error { return c.CreateNamespace(ctx, "ns") },
		},
		{
			desc: "create database",
			setup: func(t *testing.T, m *testMocks) DB {
				t.Helper()
				expectOperationStats(m, "query", 1)
				expectOperationStats(m, "creating", 1)

				return newTestDBWrapper(t, map[string]any{"query": queryReply(statusOK, nil)})
			},
			run: func(ctx context.Context, c *Client) error { return c.CreateDatabase(ctx, "db") },
		},
		{
			desc: "drop namespace",
			setup: func(t *testing.T, m *testMocks) DB {
				t.Helper()
				expectOperationStats(m, "query", 1)
				expectOperationStats(m, "dropping", 1)

				return newTestDBWrapper(t, map[string]any{"query": queryReply(statusOK, nil)})
			},
			run: func(ctx context.Context, c *Client) error { return c.DropNamespace(ctx, "ns") },
		},
		{
			desc: "drop database fails",
			setup: func(t *testing.T, m *testMocks) DB {
				t.Helper()
				expectOperationStats(m, "query", 1)
				expectOperationStats(m, "dropping", 1)

				return newTestDBWrapper(t, map[string]any{"query": queryReply("ERR", "database not found")})
			},
			run:       func(ctx context.Context, c *Client) error { return c.DropDatabase(ctx, "db") },
			expErrMsg: "query failed: database not found",
		},
		{
			desc:      "not connected",
			setup:     func(*testing.T, *testMocks) DB { return nil },
			run:       func(ctx context.Context, c *Client) error { return c.CreateNamespace(ctx, "ns") },
			expErrMsg: errNotConnected.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			client := newTestClient(m, tc.setup(t, m))

			err := tc.run(t.Context(), client)

			assert.Equal(t, tc.expErrMsg, errMsg(err))
		})
	}
}

func Test_processQueryResults(t *testing.T) {
	tests := []struct {
		desc       string
		query      string
		results    []surrealdb.QueryResult[any]
		setupMocks func(m *testMocks)
		expResult  []any
		expErr     error
	}{
		{
			desc:  "records, scalar and none results are collected",
			query: "SELECT * FROM users",
			results: []surrealdb.QueryResult[any]{
				{Status: statusOK, Result: []any{map[string]any{"id": "1", "age": uint64(30)}}},
				{Status: statusOK, Result: nil},
				{Status: statusOK, Result: models.CustomNil{}},
				{Status: statusOK, Result: "scalar"},
			},
			setupMocks: func(*testMocks) {},
			expResult:  []any{map[string]any{"id": "1", "age": 30}, true, "scalar"},
		},
		{
			desc:  "non-map entries in a record list are skipped",
			query: "SELECT * FROM users",
			results: []surrealdb.QueryResult[any]{
				{Status: statusOK, Result: []any{"not-a-record", map[string]any{"id": "2"}}},
			},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Errorf("failed to extract record: %v", errUnexpectedResult)
			},
			expResult: []any{map[string]any{"id": "2"}},
		},
		{
			desc:  "statement error fails a data query",
			query: "SELECT * FROM users",
			results: []surrealdb.QueryResult[any]{
				{Status: "ERR", Error: &surrealdb.QueryError{Message: "table missing"}},
			},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Errorf("query error: %v", "table missing")
			},
			expErr: errQueryError,
		},
		{
			desc:  "statement error is ignored for administrative queries",
			query: "DEFINE NAMESPACE ns;",
			results: []surrealdb.QueryResult[any]{
				{Status: "ERR", Error: &surrealdb.QueryError{Message: "already exists"}},
				{Status: statusOK, Result: models.CustomNil{}},
			},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Errorf("query error: %v", "already exists")
			},
			expResult: []any{true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			tc.setupMocks(m)

			client := newTestClient(m, nil)

			result, err := client.processQueryResults(tc.query, tc.results)

			assert.Equal(t, tc.expResult, result)
			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_HealthCheck(t *testing.T) {
	baseDetails := func(extra map[string]any) map[string]any {
		details := map[string]any{"host": "localhost:8000", "namespace": testNamespace, "database": testDatabase}
		for k, v := range extra {
			details[k] = v
		}

		return details
	}

	tests := []struct {
		desc      string
		setup     func(m *testMocks) DB
		expHealth *Health
		expErr    error
	}{
		{
			desc: "down: not connected",
			setup: func(m *testMocks) DB {
				expectOperationStats(m, "health_check", 0)
				return nil
			},
			expHealth: &Health{Status: "DOWN", Details: baseDetails(map[string]any{"error": "Database client is not connected"})},
			expErr:    errNotConnected,
		},
		{
			desc: "down: info fails",
			setup: func(m *testMocks) DB {
				m.db.EXPECT().Info(gomock.Any()).Return(nil, errInfo)
				expectOperationStats(m, "health_check", 1)

				return m.db
			},
			expHealth: &Health{Status: "DOWN", Details: baseDetails(map[string]any{
				"error": "Connection test failed: info failed", "connection_state": "failed",
			})},
			expErr: errInfo,
		},
		{
			desc: "down: namespace access fails",
			setup: func(m *testMocks) DB {
				m.db.EXPECT().Info(gomock.Any()).Return(map[string]any{}, nil)
				m.db.EXPECT().Use(gomock.Any(), testNamespace, testDatabase).Return(errUse)
				expectOperationStats(m, "health_check", 1)

				return m.db
			},
			expHealth: &Health{Status: "DOWN", Details: baseDetails(map[string]any{
				"error": "Database access verification failed: use failed", "connection_state": "connected_but_access_failed",
			})},
			expErr: errUse,
		},
		{
			desc: "up: fully connected",
			setup: func(m *testMocks) DB {
				m.db.EXPECT().Info(gomock.Any()).Return(map[string]any{}, nil)
				m.db.EXPECT().Use(gomock.Any(), testNamespace, testDatabase).Return(nil)
				expectOperationStats(m, "health_check", 1)

				return m.db
			},
			expHealth: &Health{Status: "UP", Details: baseDetails(map[string]any{
				"message": "Database is healthy", "connection_state": "fully_connected",
			})},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			client := newTestClient(m, tc.setup(m))

			health, err := client.HealthCheck(t.Context())

			assert.Equal(t, tc.expHealth, health)
			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_setupNamespaceAndDatabase(t *testing.T) {
	tests := []struct {
		desc       string
		setupMocks func(m *testMocks)
		expErr     error
	}{
		{
			desc: "success",
			setupMocks: func(m *testMocks) {
				m.db.EXPECT().Use(gomock.Any(), testNamespace, testDatabase).Return(nil)
			},
		},
		{
			desc: "use fails",
			setupMocks: func(m *testMocks) {
				m.db.EXPECT().Use(gomock.Any(), testNamespace, testDatabase).Return(errUse)
				m.logger.EXPECT().Errorf("%s: %v", "unable to set the namespace and database for SurrealDB", errUse)
			},
			expErr: errUse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			tc.setupMocks(m)

			err := newTestClient(m, m.db).setupNamespaceAndDatabase(t.Context())

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_authenticateCredentials(t *testing.T) {
	tests := []struct {
		desc       string
		username   string
		password   string
		setupMocks func(m *testMocks)
		expErr     error
	}{
		{
			desc:       "no credentials skips sign in",
			setupMocks: func(*testMocks) {},
		},
		{
			desc:       "only username provided",
			username:   "root",
			setupMocks: func(*testMocks) {},
			expErr:     errInvalidCredentialsConfig,
		},
		{
			desc:       "only password provided",
			password:   "secret",
			setupMocks: func(*testMocks) {},
			expErr:     errInvalidCredentialsConfig,
		},
		{
			desc:     "sign in succeeds",
			username: "root",
			password: "secret",
			setupMocks: func(m *testMocks) {
				m.db.EXPECT().SignIn(gomock.Any(), &surrealdb.Auth{Username: "root", Password: "secret"}).Return("token", nil)
			},
		},
		{
			desc:     "sign in fails",
			username: "root",
			password: "wrong",
			setupMocks: func(m *testMocks) {
				m.db.EXPECT().SignIn(gomock.Any(), &surrealdb.Auth{Username: "root", Password: "wrong"}).Return("", errAuth)
				m.logger.EXPECT().Errorf("%s: %v", "failed to sign in to SurrealDB", errAuth)
			},
			expErr: errAuth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			tc.setupMocks(m)

			client := newTestClient(m, m.db)
			client.config.Username = tc.username
			client.config.Password = tc.password

			err := client.authenticateCredentials(t.Context())

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_logError(t *testing.T) {
	tests := []struct {
		desc   string
		err    error
		logger func(m *testMocks) any
	}{
		{
			desc: "with error",
			err:  errUse,
			logger: func(m *testMocks) any {
				m.logger.EXPECT().Errorf("%s: %v", "something failed", errUse)
				return m.logger
			},
		},
		{
			desc: "without error",
			logger: func(m *testMocks) any {
				m.logger.EXPECT().Errorf("%s", "something failed")
				return m.logger
			},
		},
		{
			desc:   "without logger nothing is logged",
			err:    errUse,
			logger: func(*testMocks) any { return nil },
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)

			logger := tc.logger(m)

			client := New(&Config{})
			client.UseLogger(logger)

			// The installed logger must be exactly the one supplied (nil when none), and logError must honor
			// the mock's expectations: an exact Errorf call when a logger is set, no call and no panic otherwise.
			assert.Equal(t, logger, client.logger)
			assert.NotPanics(t, func() { client.logError("something failed", tc.err) })
		})
	}
}

func Test_buildEndpoint(t *testing.T) {
	tests := []struct {
		desc        string
		tlsEnabled  bool
		expEndpoint string
	}{
		{desc: "websocket without TLS", expEndpoint: "ws://localhost:8000"},
		{desc: "https with TLS", tlsEnabled: true, expEndpoint: "https://localhost:8000"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := New(&Config{Host: "localhost", Port: 8000, TLSEnabled: tc.tlsEnabled})

			assert.Equal(t, tc.expEndpoint, client.buildEndpoint())
		})
	}
}

func Test_Connect(t *testing.T) {
	srv := httptest.NewTLSServer(surrealRPCHandler(t, map[string]any{
		"signin": "token",
	}))
	t.Cleanup(srv.Close)

	// Trust the in-process TLS server for the SurrealDB HTTP client, which uses the default transport.
	// This swaps the process-wide http.DefaultTransport, so tests in this package must not use t.Parallel.
	originalTransport := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport

	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	httpsEndpoint := fmt.Sprintf("https://%s:%d", host, port)

	tests := []struct {
		desc        string
		config      Config
		setupMocks  func(m *testMocks)
		expConnDone bool
	}{
		{
			desc:   "websocket handshake fails",
			config: Config{Host: host, Port: port, Namespace: testNamespace, Database: testDatabase},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Debugf("connecting to SurrealDB at %s", fmt.Sprintf("ws://%s:%d", host, port))
				m.logger.EXPECT().Errorf("%s: %v", "failed to connect to SurrealDB", gomock.Any())
			},
		},
		{
			desc:   "connected without credentials",
			config: Config{Host: host, Port: port, Namespace: testNamespace, Database: testDatabase, TLSEnabled: true},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Debugf("connecting to SurrealDB at %s", httpsEndpoint)
				m.logger.EXPECT().Logf("Successfully connected to SurrealDB at %v:%v to database %v", host, port, testDatabase)
			},
			expConnDone: true,
		},
		{
			desc: "connected and signed in",
			config: Config{Host: host, Port: port, Namespace: testNamespace, Database: testDatabase, TLSEnabled: true,
				Username: "root", Password: "root"},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Debugf("connecting to SurrealDB at %s", httpsEndpoint)
				m.logger.EXPECT().Logf("Successfully connected to SurrealDB at %v:%v to database %v", host, port, testDatabase)
			},
			expConnDone: true,
		},
		{
			desc: "incomplete credentials abort connection setup",
			config: Config{Host: host, Port: port, Namespace: testNamespace, Database: testDatabase, TLSEnabled: true,
				Username: "root"},
			setupMocks: func(m *testMocks) {
				m.logger.EXPECT().Debugf("connecting to SurrealDB at %s", httpsEndpoint)
			},
			expConnDone: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			m := newTestMocks(t)
			m.logger.EXPECT().Debugf("connecting to SurrealDB at %v:%v to database %v", host, port, testDatabase)
			m.metrics.EXPECT().NewHistogram("app_surrealdb_stats", gomock.Any(), gomock.Any())
			m.metrics.EXPECT().NewGauge("app_surrealdb_open_connections", gomock.Any())
			tc.setupMocks(m)

			client := New(&tc.config)
			client.UseLogger(m.logger)
			client.UseMetrics(m.metrics)

			client.Connect()

			assert.Equal(t, tc.expConnDone, client.db != nil)
		})
	}
}
