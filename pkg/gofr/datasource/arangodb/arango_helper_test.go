package arangodb

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

func Test_Client_CreateUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats",
		gomock.Any(), "endpoint", gomock.Any(), gomock.Any(), gomock.Any())

	err := client.createUser(context.Background(), "test", UserOptions{
		Password: "user123",
		Extra:    nil,
	})
	require.NoError(t, err, "Test_Arango_CreateUser: failed to create user")
}

func Test_Client_DropUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().RemoveUser(gomock.Any(), gomock.Any()).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	err := client.dropUser(context.Background(), "test")
	require.NoError(t, err, "Test_Arango_DropUser: failed to drop user")
}

func Test_Client_GrantDB(t *testing.T) {
	client, mockArango, mockUser, mockLogger, mockMetrics := setupDB(t)

	// Test data
	ctx := context.Background()
	dbName := "testDB"
	username := "testUser"

	// Expectations
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		ctx, "app_arango_stats", gomock.Any(), "endpoint",
		gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Expect user() call and return our mock user that implements the full interface
	mockArango.EXPECT().User(gomock.Any(), username).Return(mockUser, nil).MaxTimes(2)

	// Test cases
	testCases := []struct {
		name       string
		dbName     string
		username   string
		permission string
		expectErr  bool
	}{
		{
			name:       "Valid grant read-write",
			dbName:     dbName,
			username:   username,
			permission: string(arangodb.GrantReadWrite),
			expectErr:  false,
		},
		{
			name:       "Valid grant read-only",
			dbName:     dbName,
			username:   username,
			permission: string(arangodb.GrantReadOnly),
			expectErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.grantDB(ctx, tc.dbName, tc.username, tc.permission)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_Client_GrantDB_Errors(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	dbName := "testDB"
	username := "testUser"

	// Expect user() call to return error
	mockArango.EXPECT().User(gomock.Any(), username).Return(nil, errUserNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(ctx, "app_arango_stats", gomock.Any(), "endpoint",
		gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.grantDB(ctx, dbName, username, string(arangodb.GrantReadWrite))
	require.Error(t, err)
}

func TestUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockArango := NewMockClient(ctrl)
	mockUser := NewMockUser(ctrl)
	client := &Client{client: mockArango}

	ctx := context.Background()
	username := "testUser"

	t.Run("Successful user fetch", func(t *testing.T) {
		mockArango.EXPECT().
			User(ctx, username).
			Return(mockUser, nil)

		user, err := client.user(ctx, username)
		require.NoError(t, err)
		require.NotNil(t, user)
	})

	t.Run("user fetch error", func(t *testing.T) {
		mockArango.EXPECT().
			User(ctx, username).
			Return(nil, errUserNotFound)

		user, err := client.user(ctx, username)
		require.Error(t, err)
		require.Nil(t, user)
	})
}

func TestClient_Database(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockClient(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

	client.client = mockArango

	mockDatabase := NewMockDatabase(gomock.NewController(t))

	ctx := context.Background()
	dbName := "testDB"

	t.Run("Get database Success", func(t *testing.T) {
		mockArango.EXPECT().
			GetDatabase(ctx, dbName, nil).
			Return(mockDatabase, nil)
		mockDatabase.EXPECT().Name().Return(dbName)

		db, err := client.database(ctx, dbName)
		require.NoError(t, err)
		require.NotNil(t, db)
		require.Equal(t, dbName, db.Name())
	})

	t.Run("Get database Error", func(t *testing.T) {
		mockArango.EXPECT().
			GetDatabase(ctx, dbName, nil).
			Return(nil, errDBNotFound)

		db, err := client.database(ctx, dbName)
		require.Error(t, err)
		require.Nil(t, db)
	})

	// Test database operations
	t.Run("database Operations", func(t *testing.T) {
		mockArango.EXPECT().
			GetDatabase(ctx, dbName, nil).
			Return(mockDatabase, nil)
		mockDatabase.EXPECT().Name().Return(dbName)
		mockDatabase.EXPECT().Remove(ctx).Return(nil)
		mockDatabase.EXPECT().GetCollection(ctx, "testCollection", nil).
			Return(nil, nil)

		db, err := client.database(ctx, dbName)
		require.NoError(t, err)
		require.Equal(t, dbName, db.Name())

		err = db.Remove(ctx)
		require.NoError(t, err)

		coll, err := db.GetCollection(ctx, "testCollection", nil)
		require.NoError(t, err)
		require.Nil(t, coll)
	})
}

func Test_Client_GrantCollection(t *testing.T) {
	client, mockArango, mockUser, mockLogger, mockMetrics := setupDB(t)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(
		context.Background(), "app_arango_stats", gomock.Any(), "endpoint",
		gomock.Any(), gomock.Any(), gomock.Any())

	mockArango.EXPECT().User(gomock.Any(), "testUser").Return(mockUser, nil)

	err := client.grantCollection(context.Background(), "testDB", "testCollection",
		"testUser", string(arangodb.GrantReadOnly))

	require.NoError(t, err)
}

func Test_Client_GrantCollection_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		context.Background(), "app_arango_stats", gomock.Any(), "endpoint",
		gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockArango.EXPECT().User(gomock.Any(), "testUser").Return(nil, errUserNotFound)

	err := client.grantCollection(context.Background(), "testDB", "testCollection",
		"testUser", string(arangodb.GrantReadOnly))

	require.ErrorIs(t, errUserNotFound, err, "Expected error when user not found")
}
