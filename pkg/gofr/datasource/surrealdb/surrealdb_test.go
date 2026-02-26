package surrealdb

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
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
