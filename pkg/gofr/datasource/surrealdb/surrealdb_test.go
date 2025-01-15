package surrealdb

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"

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

	t.Run("invalid logger", func(t *testing.T) {
		client.UseLogger("not a logger")
		assert.NotNil(t, client.logger) // Should retain previous valid logger
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

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful query", func(t *testing.T) {
		ctx := context.Background()
		query := "SELECT * FROM users"

		queryResult := []QueryResult{
			{
				Status: "OK",
				Result: []interface{}{
					map[string]interface{}{
						"id":   "user:123",
						"name": "test",
					},
				},
			},
		}

		var resp QueryResponse
		resp.Result = &queryResult
		mockConn.EXPECT().Send(gomock.Any(), "query", query, nil).Return(nil).SetArg(0, resp)

		results, err := client.Query(ctx, query, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("query with error", func(t *testing.T) {
		ctx := context.Background()
		query := "INVALID QUERY"

		mockConn.EXPECT().Send(gomock.Any(), "query", query, nil).Return(errInvalidQuery)

		results, err := client.Query(ctx, query, nil)
		require.Error(t, err)
		require.Nil(t, results)
	})
}

var errInsert = errors.New("insert error")

func Test_Insert(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful insert", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]interface{}{
			"name":  "test",
			"email": "test@example.com",
		}

		insertResponse := Response{
			Result: map[string]interface{}{
				"id":    "user:123",
				"name":  "test",
				"email": "test@example.com",
			},
		}

		mockConn.EXPECT().Send(gomock.Any(), "insert", "users", data).Return(nil).SetArg(0, insertResponse)

		result, err := client.Insert(ctx, "users", data)
		require.NoError(t, err)
		assert.NotNil(t, result)

		if resultMap, ok := result.Result.(map[string]interface{}); ok {
			assert.Equal(t, "test", resultMap["name"])
			assert.Equal(t, "test@example.com", resultMap["email"])
		} else {
			t.Errorf("Result is not of type map[string]interface{}")
		}
	})

	t.Run("insert error", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]interface{}{
			"name": "test",
		}

		mockConn.EXPECT().Send(gomock.Any(), "insert", "users", data).Return(errInsert)

		result, err := client.Insert(ctx, "users", data)
		require.Error(t, err)
		require.Nil(t, result)
	})
}

func Test_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful create", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]interface{}{
			"name":  "test",
			"email": "test@example.com",
		}

		expectedResponse := Response{
			Result: map[string]interface{}{
				"id":    "user:123",
				"name":  "test",
				"email": "test@example.com",
			},
		}

		mockConn.EXPECT().Send(gomock.Any(), "create", "users", data).Return(nil).SetArg(0, expectedResponse)

		result, err := client.Create(ctx, "users", data)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, "test@example.com", result["email"])
	})

	t.Run("database error", func(t *testing.T) {
		ctx := context.Background()
		client.db = mockConn

		data := map[string]interface{}{
			"name": "test",
		}

		dbError := errorDatabase
		mockConn.EXPECT().Send(gomock.Any(), "create", "users", data).Return(dbError)

		result, err := client.Create(ctx, "users", data)
		require.Error(t, err)
		require.Equal(t, dbError, err)
		require.Nil(t, result)
	})

	t.Run("unexpected result type error", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]interface{}{
			"name": "test",
		}

		// Return a string instead of map[string]interface{}
		unexpectedResponse := Response{
			Result: "unexpected type",
		}

		mockConn.EXPECT().Send(gomock.Any(), "create", "users", data).Return(nil).SetArg(0, unexpectedResponse)

		result, err := client.Create(ctx, "users", data)
		require.Error(t, err)
		require.ErrorIs(t, err, errUnexpectedResultType)
		require.Nil(t, result)
	})
}

func Test_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful update", func(t *testing.T) {
		ctx := context.Background()
		data := map[string]interface{}{
			"name": "updated",
		}

		updateResponse := Response{
			Result: []interface{}{
				map[interface{}]interface{}{
					"id":   "user:123",
					"name": "updated",
				},
			},
		}

		mockConn.EXPECT().Send(gomock.Any(), "update", "users", data).Return(nil).SetArg(0, updateResponse)

		result, err := client.Update(ctx, "users", "123", data)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("not connected error", func(t *testing.T) {
		ctx := context.Background()
		client.db = nil

		data := map[string]interface{}{
			"name": "updated",
		}

		result, err := client.Update(ctx, "users", "123", data)
		require.Error(t, err)
		require.Equal(t, errNotConnected, err)
		require.Nil(t, result)

		client.db = mockConn
	})
}

func Test_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful delete", func(t *testing.T) {
		ctx := context.Background()
		table := "users"
		id := "123"

		deleteResponse := Response{
			Result: map[string]interface{}{
				"id":      "user:123",
				"deleted": true,
			},
		}

		arg := models.RecordID{
			Table: table,
			ID:    id,
		}

		mockConn.EXPECT().Send(gomock.Any(), "delete", arg).Return(nil).SetArg(0, deleteResponse)

		result, err := client.Delete(ctx, table, id)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("empty record ID", func(t *testing.T) {
		ctx := context.Background()
		table := "users"
		id := "" // Empty ID

		arg := models.RecordID{
			Table: table,
			ID:    id,
		}

		deleteResponse := Response{
			Result: nil,
		}

		mockConn.EXPECT().Send(gomock.Any(), "delete", arg).Return(nil).SetArg(0, deleteResponse)

		result, err := client.Delete(ctx, table, id)
		require.NoError(t, err)
		require.Nil(t, result)
	})
}
func Test_Select(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful select", func(t *testing.T) {
		ctx := context.Background()
		selectResponse := Response{
			Result: []interface{}{
				map[interface{}]interface{}{
					"id":    "user:123",
					"name":  "test",
					"email": "test@example.com",
				},
			},
		}

		mockConn.EXPECT().Send(gomock.Any(), "select", "users").Return(nil).SetArg(0, selectResponse)

		results, err := client.Select(ctx, "users")
		require.NoError(t, err)
		assert.NotEmpty(t, results)
		assert.Equal(t, "test", results[0]["name"])
	})

	t.Run("empty result set", func(t *testing.T) {
		ctx := context.Background()
		emptyResponse := Response{
			Result: []interface{}{},
		}

		mockConn.EXPECT().Send(gomock.Any(), "select", "users").Return(nil).SetArg(0, emptyResponse)

		results, err := client.Select(ctx, "users")
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func Test_SignIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockConnection(ctrl)
	client := New(&Config{})
	client.db = mockConn

	t.Run("successful signin", func(t *testing.T) {
		authData := &surrealdb.Auth{
			Username: "test",
			Password: "test",
		}

		var tokenResp connection.RPCResponse[string]
		tokenResp.Result = new(string)
		*tokenResp.Result = "token123"

		mockConn.EXPECT().Send(gomock.Any(), "signin", authData).Return(nil).SetArg(0, tokenResp)
		mockConn.EXPECT().Let(gomock.Any(), gomock.Any()).Return(nil)

		token, err := client.SignIn(authData)
		require.NoError(t, err)
		assert.Equal(t, "token123", token)
	})
}

var errConnection = errors.New("connection error")

func Test_HealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockConn := NewMockConnection(ctrl)

	client := New(&Config{
		Host:      "localhost",
		Port:      8000,
		Namespace: "test",
		Database:  "test",
	})
	client.UseLogger(mockLogger)
	client.db = mockConn

	t.Run("successful health check", func(t *testing.T) {
		ctx := context.Background()

		queryResult := []QueryResult{
			{
				Status: "OK",
				Result: []interface{}{"SurrealDB Health Check"},
			},
		}

		var resp QueryResponse
		resp.Result = &queryResult
		mockConn.EXPECT().Send(gomock.Any(), "query", "RETURN 'SurrealDB Health Check'", nil).Return(nil).SetArg(0, resp)

		result, err := client.HealthCheck(ctx)
		require.NoError(t, err)

		health, ok := result.(*Health)
		require.True(t, ok)
		assert.Equal(t, "UP", health.Status)
	})

	t.Run("failed health check", func(t *testing.T) {
		ctx := context.Background()

		mockConn.EXPECT().Send(gomock.Any(), "query", "RETURN 'SurrealDB Health Check'", nil).
			Return(errConnection)

		result, err := client.HealthCheck(ctx)
		require.Error(t, err)

		health, ok := result.(*Health)
		require.True(t, ok)
		require.Equal(t, "DOWN", health.Status)
	})
}
