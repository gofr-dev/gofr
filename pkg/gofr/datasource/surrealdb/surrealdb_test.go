// TODO: Tests need to be updated for SurrealDB client v1.0.0
// The old Connection interface mock is no longer applicable as we now use *surrealdb.DB directly
//go:build integration

package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

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
}

func Test_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{})
	mockLogger := NewMockLogger(ctrl)

	t.Run("valid logger", func(t *testing.T) {
		client.UseLogger(mockLogger)
		assert.NotNil(t, client.logger)
	})
}

var errorDatabase = errors.New("database error")

func Test_useNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful namespace switch", func(t *testing.T) {
		mockConn.EXPECT().Use("test_namespace", "").Return(nil)

		err := client.useNamespace("test_namespace")
		require.NoError(t, err)
	})

	t.Run("nil database connection", func(t *testing.T) {
		client.db = nil

		err := client.useNamespace("test_namespace")
		assert.ErrorIs(t, err, errNotConnected)
	})

	t.Run("database error", func(t *testing.T) {
		client.db = mockConn

		mockConn.EXPECT().Use("test_namespace", "").Return(errorDatabase)

		err := client.useNamespace("test_namespace")
		require.Error(t, err)
		assert.Equal(t, errorDatabase, err)
	})
}

func Test_useDatabase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful database switch", func(t *testing.T) {
		mockConn.EXPECT().Use("", "test_database").Return(nil)

		err := client.useDatabase("test_database")
		require.NoError(t, err)
	})

	t.Run("nil database connection", func(t *testing.T) {
		client.db = nil

		err := client.useDatabase("test_database")
		assert.ErrorIs(t, err, errNotConnected)
	})

	t.Run("database error", func(t *testing.T) {
		client.db = mockConn
		expectedErr := errorDatabase

		mockConn.EXPECT().Use("", "test_database").Return(expectedErr)

		err := client.useDatabase("test_database")
		require.Error(t, err)
		require.Equal(t, expectedErr, err)
	})
}

var errInvalidQuery = errors.New("invalid query")

func Test_Query(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-surrealdb"))
	client.db = mockConn

	t.Run("successful query", func(t *testing.T) {
		ctx := context.Background()
		query := "SELECT * FROM users"

		queryResult := []QueryResult{
			{
				Status: statusOK,
				Time:   "0.000s",
				Result: []any{
					map[any]any{
						"id":    "user:1",
						"name":  "test1",
						"email": "test1@example.com",
					},
				},
			},
		}

		var resp QueryResponse

		resp.Result = &queryResult

		mockConn.EXPECT().
			Send(gomock.Any(), "query", query, nil).
			Return(nil).
			SetArg(0, resp)

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		results, err := client.Query(ctx, query, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, results)

		// Verify the extracted and processed result
		resultMap, ok := results[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test1", resultMap["name"])
		assert.Equal(t, "test1@example.com", resultMap["email"])
	})

	t.Run("query with error", func(t *testing.T) {
		ctx := context.Background()
		query := "INVALID QUERY"

		mockConn.EXPECT().
			Send(gomock.Any(), "query", query, nil).
			Return(errInvalidQuery)

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		results, err := client.Query(ctx, query, nil)
		require.Error(t, err)
		assert.Nil(t, results)
	})
}

func Test_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("successful create", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]any{
			"name":  "test",
			"email": "test@example.com",
		}

		createResponse := DBResponse{
			Result: map[any]any{
				"id":    "user:123",
				"name":  "test",
				"email": "test@example.com",
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		mockConn.EXPECT().
			Send(gomock.Any(), "create", "users", data).
			Return(nil).
			SetArg(0, createResponse)

		result, err := client.Create(ctx, "users", data)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, "test@example.com", result["email"])
	})

	t.Run("database error", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]any{"name": "test"}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		mockConn.EXPECT().
			Send(gomock.Any(), "create", "users", data).
			Return(errorDatabase)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		result, err := client.Create(ctx, "users", data)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func Test_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("successful update", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]any{
			"name":  "updated",
			"age":   25,
			"email": "test@example.com",
		}

		expectedQuery := fmt.Sprintf("UPDATE %s:%s MERGE $data RETURN *", "users", "123")

		queryVars := map[string]any{
			"data": data,
		}

		updateResponse := DBResponse{
			Result: []any{
				map[any]any{
					"status": statusOK,
					"result": map[any]any{
						"id":    "user:123",
						"name":  "updated",
						"age":   25,
						"email": "test@example.com",
					},
				},
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		mockConn.EXPECT().
			Send(gomock.Any(), "query", expectedQuery, queryVars).
			Return(nil).
			SetArg(0, updateResponse)

		result, err := client.Update(ctx, "users", "123", data)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "updated", resultMap["name"])
		assert.Equal(t, 25, resultMap["age"])
		assert.Equal(t, "test@example.com", resultMap["email"])
	})

	t.Run("not connected error", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		data := map[string]any{"name": "updated"}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		result, err := client.Update(ctx, "users", "123", data)
		require.Error(t, err)
		require.ErrorIs(t, err, errNotConnected)
		require.Nil(t, result)

		client.db = mockConn
	})
}

func Test_Select(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-surrealdb"))
	client.db = mockConn

	t.Run("successful select", func(t *testing.T) {
		ctx := context.Background()

		// Use DBResponse for select operation
		selectResponse := DBResponse{
			Result: []any{
				map[any]any{
					"id":    "user:123",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		mockConn.EXPECT().
			Send(gomock.Any(), "select", "users").
			Return(nil).
			SetArg(0, selectResponse)

		results, err := client.Select(ctx, "users")
		require.NoError(t, err)
		assert.NotEmpty(t, results)
		assert.Equal(t, "test", results[0]["name"])
		assert.Equal(t, "test@example.com", results[0]["email"])
	})

	t.Run("empty result set", func(t *testing.T) {
		ctx := context.Background()

		emptyResponse := DBResponse{
			Result: []any{},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))

		mockConn.EXPECT().
			Send(gomock.Any(), "select", "users").
			Return(nil).
			SetArg(0, emptyResponse)

		results, err := client.Select(ctx, "users")
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func Test_Insert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("successful insert", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]any{
			"name":  "test",
			"email": "test@example.com",
		}

		mockResponse := DBResponse{
			Result: []any{
				map[string]any{
					"id":    "user:1",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))
		mockConn.EXPECT().
			Send(gomock.Any(), "insert", "users", data).
			Return(nil).
			SetArg(0, mockResponse)

		result, err := client.Insert(ctx, "users", data)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test", result[0]["name"])
		assert.Equal(t, "test@example.com", result[0]["email"])
	})

	t.Run("not connected error", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		result, err := client.Insert(ctx, "users", nil)
		require.ErrorIs(t, err, errNotConnected)
		require.Nil(t, result)

		client.db = mockConn
	})

	t.Run("unexpected result type error", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]any{"name": "test"}

		mockResponse := DBResponse{
			Result: "invalid result type",
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))
		mockConn.EXPECT().
			Send(gomock.Any(), "insert", "users", data).
			Return(nil).
			SetArg(0, mockResponse)

		result, err := client.Insert(ctx, "users", data)
		require.ErrorIs(t, err, errUnexpectedResultType)
		require.Nil(t, result)
	})
}

func Test_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("successful delete", func(t *testing.T) {
		ctx := context.Background()
		expectedQuery := "DELETE FROM users:123 RETURN BEFORE;"

		mockResponse := DBResponse{
			Result: []any{
				map[any]any{
					"id":    "user:123",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))
		mockConn.EXPECT().
			Send(gomock.Any(), "query", expectedQuery, nil).
			Return(nil).
			SetArg(0, mockResponse)

		result, err := client.Delete(ctx, "users", "123")
		require.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test", resultMap["name"])
		assert.Equal(t, "test@example.com", resultMap["email"])
	})

	t.Run("not connected error", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		result, err := client.Delete(ctx, "users", "123")
		require.ErrorIs(t, err, errNotConnected)
		require.Nil(t, result)

		client.db = mockConn
	})

	t.Run("no results", func(t *testing.T) {
		ctx := context.Background()
		expectedQuery := "DELETE FROM users:123 RETURN BEFORE;"

		mockResponse := DBResponse{
			Result: []any{},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any())
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1))
		mockConn.EXPECT().
			Send(gomock.Any(), "query", expectedQuery, nil).
			Return(nil).
			SetArg(0, mockResponse)

		result, err := client.Delete(ctx, "users", "123")
		require.NoError(t, err)
		require.Nil(t, result)
	})
}

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
			{"at max boundary", int64(math.MaxInt), int(math.MaxInt)},
			{"at min boundary", int64(math.MinInt), int(math.MinInt)},
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
}

func Test_executeQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("successful query execution", func(t *testing.T) {
		ctx := context.Background()
		queryResult := []QueryResult{{
			Status: statusOK,
			Result: []any{
				map[any]any{
					"id":    "test:1",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "TEST QUERY", nil).
			Return(nil).
			SetArg(0, QueryResponse{Result: &queryResult})

		err := client.executeQuery(ctx, "Test", "entity", "TEST QUERY")
		require.NoError(t, err)
	})

	t.Run("not connected error", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		err := client.executeQuery(ctx, "Test", "entity", "TEST QUERY")
		require.ErrorIs(t, err, errNotConnected)

		client.db = mockConn
	})

	t.Run("query execution error", func(t *testing.T) {
		ctx := context.Background()

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "TEST QUERY", nil).
			Return(errorDatabase)

		err := client.executeQuery(ctx, "Test", "entity", "TEST QUERY")
		require.Error(t, err)
	})
}

func Test_NamespaceOperations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("create namespace success", func(t *testing.T) {
		ctx := context.Background()
		queryResult := []QueryResult{{
			Status: statusOK,
			Result: []any{
				map[any]any{
					"id":    "test:1",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "DEFINE NAMESPACE test_namespace;", nil).
			Return(nil).
			SetArg(0, QueryResponse{Result: &queryResult})

		err := client.CreateNamespace(ctx, "test_namespace")
		require.NoError(t, err)
	})

	t.Run("drop namespace success", func(t *testing.T) {
		ctx := context.Background()
		queryResult := []QueryResult{{
			Status: statusOK,
			Result: []any{},
		}}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "REMOVE NAMESPACE test_namespace;", nil).
			Return(nil).
			SetArg(0, QueryResponse{Result: &queryResult})

		err := client.DropNamespace(ctx, "test_namespace")
		require.NoError(t, err)
	})
}

func Test_DatabaseOperations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := New(&Config{
		Namespace: "test_ns",
		Database:  "test_db",
	})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.db = mockConn

	t.Run("create database success", func(t *testing.T) {
		ctx := context.Background()
		queryResult := []QueryResult{{
			Status: statusOK,
			Result: []any{
				map[any]any{
					"id":    "test:1",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "DEFINE DATABASE test_database;", nil).
			Return(nil).
			SetArg(0, QueryResponse{Result: &queryResult})

		err := client.CreateDatabase(ctx, "test_database")
		require.NoError(t, err)
	})

	t.Run("drop database success", func(t *testing.T) {
		ctx := context.Background()
		queryResult := []QueryResult{{
			Status: statusOK,
			Result: []any{},
		}}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
		mockMetrics.EXPECT().SetGauge("app_surrealdb_open_connections", float64(1)).MaxTimes(2)
		mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_surrealdb_stats", gomock.Any(), gomock.Any()).MaxTimes(2)
		mockConn.EXPECT().
			Send(gomock.Any(), "query", "REMOVE DATABASE test_database;", nil).
			Return(nil).
			SetArg(0, QueryResponse{Result: &queryResult})

		err := client.DropDatabase(ctx, "test_database")
		require.NoError(t, err)
	})

	t.Run("database operations when not connected", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		err := client.CreateDatabase(ctx, "test_database")
		require.ErrorIs(t, err, errNotConnected)

		err = client.DropDatabase(ctx, "test_database")
		require.ErrorIs(t, err, errNotConnected)

		client.db = mockConn
	})
}
