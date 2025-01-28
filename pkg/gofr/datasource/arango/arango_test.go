package arango

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errUserNotFound       = errors.New("user not found")
	errDBNotFound         = errors.New("database not found")
	errCollectionNotFound = errors.New("collection not found")
	errDocumentNotFound   = errors.New("document not found")
)

func setupDB(t *testing.T) (*Client, *MockArango, *MockUser, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockArango(ctrl)
	mockUser := NewMockUser(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

	client.client = mockArango

	return client, mockArango, mockUser, mockLogger, mockMetrics
}

func Test_NewArangoClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Debugf(gomock.Any(), gomock.Any())
	logger.EXPECT().Logf(gomock.Any(), gomock.Any())

	metrics.EXPECT().NewHistogram("app_arango_stats",
		"Response time of ArangoDB operations in milliseconds.", gomock.Any())

	client := New(Config{Host: "localhost", Port: 8529, Password: "root", User: "admin"})

	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	require.NotNil(t, client)
}

func Test_Client_CreateUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().AddUser(gomock.Any(), "test", &arangodb.UserOptions{}).Return(nil, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats",
		gomock.Any(), "endpoint", gomock.Any(), gomock.Any(), gomock.Any())

	err := client.CreateUser(context.Background(), "test", &arangodb.UserOptions{})
	require.NoError(t, err, "Test_Arango_CreateUser: failed to create user")
}

func Test_Client_DropUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	// Ensure the mock returns nil as an error type
	mockArango.EXPECT().DropUser(gomock.Any(), "test").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	err := client.DropUser(context.Background(), "test")
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

	// Expect User() call and return our mock user that implements the full interface
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
			err := client.GrantDB(ctx, tc.dbName, tc.username, tc.permission)
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

	// Expect User() call to return error
	mockArango.EXPECT().User(gomock.Any(), username).Return(nil, errUserNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(ctx, "app_arango_stats", gomock.Any(), "endpoint",
		gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.GrantDB(ctx, dbName, username, string(arangodb.GrantReadWrite))
	require.Error(t, err)
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid config",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: false,
		},
		{
			name: "Empty host",
			config: Config{
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: host is empty",
		},
		{
			name: "Empty port",
			config: Config{
				Host:     "localhost",
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: port is empty",
		},
		{
			name: "Empty user",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: user is empty",
		},
		{
			name: "Empty password",
			config: Config{
				Host: "localhost",
				Port: 8529,
				User: "root",
			},
			expectErr: true,
			errMsg:    "missing required field in config: password is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{config: &tc.config}
			err := client.validateConfig()

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockArango := NewMockArango(ctrl)
	mockUser := NewMockUser(ctrl)
	client := &Client{client: mockArango}

	ctx := context.Background()
	username := "testUser"

	t.Run("Successful user fetch", func(t *testing.T) {
		mockArango.EXPECT().
			User(ctx, username).
			Return(mockUser, nil)

		user, err := client.User(ctx, username)
		require.NoError(t, err)
		require.NotNil(t, user)
	})

	t.Run("User fetch error", func(t *testing.T) {
		mockArango.EXPECT().
			User(ctx, username).
			Return(nil, errUserNotFound)

		user, err := client.User(ctx, username)
		require.Error(t, err)
		require.Nil(t, user)
	})
}

func TestClient_Database(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockArango(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

	client.client = mockArango

	mockDatabase := NewMockDatabase(gomock.NewController(t))

	ctx := context.Background()
	dbName := "testDB"

	t.Run("Get Database Success", func(t *testing.T) {
		mockArango.EXPECT().
			Database(ctx, dbName).
			Return(mockDatabase, nil)
		mockDatabase.EXPECT().Name().Return(dbName)

		db, err := client.Database(ctx, dbName)
		require.NoError(t, err)
		require.NotNil(t, db)
		require.Equal(t, dbName, db.Name())
	})

	t.Run("Get Database Error", func(t *testing.T) {
		mockArango.EXPECT().
			Database(ctx, dbName).
			Return(nil, errDBNotFound)

		db, err := client.Database(ctx, dbName)
		require.Error(t, err)
		require.Nil(t, db)
	})

	// Test database operations
	t.Run("Database Operations", func(t *testing.T) {
		mockArango.EXPECT().
			Database(ctx, dbName).
			Return(mockDatabase, nil)
		mockDatabase.EXPECT().Name().Return(dbName)
		mockDatabase.EXPECT().Remove(ctx).Return(nil)
		mockDatabase.EXPECT().Collection(ctx, "testCollection").Return(nil, nil)

		db, err := client.Database(ctx, dbName)
		require.NoError(t, err)
		require.Equal(t, dbName, db.Name())

		err = db.Remove(ctx)
		require.NoError(t, err)

		coll, err := db.Collection(ctx, "testCollection")
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

	err := client.GrantCollection(context.Background(), "testDB", "testCollection",
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

	err := client.GrantCollection(context.Background(), "testDB", "testCollection",
		"testUser", string(arangodb.GrantReadOnly))

	require.ErrorIs(t, errUserNotFound, err, "Expected error when user not found")
}
