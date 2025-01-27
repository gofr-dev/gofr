package surrealdb

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		expectedQuery := `
        UPDATE users:123 SET 
        name = $name, 
        age = $age, 
        email = $email
        RETURN *`

		updateResponse := DBResponse{
			Result: []any{
				map[any]any{
					"id":    "user:123",
					"name":  "updated",
					"age":   25,
					"email": "test@example.com",
				},
			},
		}

		mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

		mockConn.EXPECT().
			Send(gomock.Any(), "query", expectedQuery, data).
			Return(nil).
			SetArg(0, updateResponse)

		result, err := client.Update(ctx, "users", "123", data)
		require.NoError(t, err)

		resultMap, ok := result.(map[any]any)
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

		mockConn.EXPECT().
			Send(gomock.Any(), "select", "users").
			Return(nil).
			SetArg(0, emptyResponse)

		results, err := client.Select(ctx, "users")
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
