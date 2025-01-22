package arango

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errUserNotFound       = errors.New("user not found")
	errDBNotFound         = errors.New("database not found")
	errCollectionNotFound = errors.New("collection not found")
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
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arango"))

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

	assert.NotNil(t, client)
}

func Test_Arango_CreateUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	mockArango.EXPECT().CreateUser(gomock.Any(), "test", gomock.Any()).Return(nil, nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats",
		gomock.Any(), "endpoint", gomock.Any(), gomock.Any(), gomock.Any())

	_, err := client.CreateUser(context.Background(), "test", nil)
	require.NoError(t, err, "Test_Arango_CreateUser: failed to create user")
}

func Test_Arango_DropUser(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	// Ensure the mock returns nil as an error type
	mockArango.EXPECT().DropUser(gomock.Any(), "test").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(context.Background(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	err := client.DropUser(context.Background(), "test")
	require.NoError(t, err, "Test_Arango_DropUser: failed to drop user")
}

func TestGrantDB(t *testing.T) {
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

func TestGrantDB_Errors(t *testing.T) {
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
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arango"))

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

func Test_Client_ListDBs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockArango(ctrl)

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arango"))

	client.client = mockArango

	ctx := context.Background()
	dbNames := []string{"db1", "db2", "db3"}
	mockDatabases := make([]arangodb.Database, len(dbNames))

	// Initialize the mock databases using the single mock controller
	for i, name := range dbNames {
		mockDatabase := NewMockDatabase(ctrl)
		mockDatabase.EXPECT().Name().Return(name).AnyTimes()
		mockDatabases[i] = mockDatabase
	}

	// Expectations
	mockArango.EXPECT().Databases(gomock.Any()).Return(mockDatabases, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Execute
	names, err := client.ListDBs(ctx)
	require.NoError(t, err)
	require.Equal(t, names[3:], dbNames)
}

func Test_Client_CreateDB(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "testDB"

	mockArango.EXPECT().CreateDatabase(gomock.Any(), database, nil).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateDB(ctx, database)
	require.NoError(t, err, "Expected no error while creating the database")
}

func Test_Client_CreateDB_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "errorDB"

	mockArango.EXPECT().CreateDatabase(gomock.Any(), database, nil).Return(errDBNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateDB(ctx, database)
	require.Error(t, err, "Expected an error while creating the database")
	require.Equal(t, "database not found", err.Error())
}

func Test_Client_DropDB(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "testDB"
	mockDB := NewMockDatabase(gomock.NewController(t))

	// Mock the Database method to return a mock database instance
	mockArango.EXPECT().Database(gomock.Any(), database).Return(mockDB, nil).Times(1)
	mockDB.EXPECT().Remove(gomock.Any()).Return(nil).Times(1) // Mock Remove to return no error
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DropDB(ctx, database)
	require.NoError(t, err, "Expected no error while dropping the database")
}

func Test_Client_DropDB_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "testDB"

	mockArango.EXPECT().Database(gomock.Any(), database).Return(nil, errDBNotFound).Times(1)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DropDB(ctx, database)
	require.Error(t, err, "Expected error when trying to drop a non-existent database")
	require.Equal(t, "database not found", err.Error())
}

func Test_Client_DropDB_RemoveError(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Remove(gomock.Any()).Return(errDBNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DropDB(context.Background(), "testDB")
	require.Error(t, err, "Expected error when removing the database")
	require.Equal(t, "database not found", err.Error())
}

func Test_Client_CreateCollection(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().CreateCollection(gomock.Any(), "testCollection", gomock.Any()).Return(nil, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateCollection(context.Background(), "testDB", "testCollection", true)
	require.NoError(t, err, "Expected no error while creating the collection")
}

func Test_Client_CreateCollection_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().CreateCollection(gomock.Any(), "testCollection", gomock.Any()).Return(nil, errCollectionNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateCollection(context.Background(), "testDB", "testCollection", false)
	require.Error(t, err, "Expected an error while creating the collection")
	require.Equal(t, "collection not found", err.Error())
}

func Test_Client_DropCollection(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().Remove(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats",
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Execute
	err := client.DropCollection(context.Background(), "testDB", "testCollection")
	require.NoError(t, err, "Expected no error while dropping the collection")
}

func Test_Client_DropCollection_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(nil, errCollectionNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DropCollection(context.Background(), "testDB", "testCollection")
	require.Error(t, err, "Expected error when trying to drop a non-existent collection")
	require.Equal(t, "collection not found", err.Error())
}

func Test_Client_TruncateCollection(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().Truncate(gomock.Any()).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.TruncateCollection(context.Background(), "testDB", "testCollection")
	require.NoError(t, err, "Expected no error while truncating the collection")
}

func Test_Client_TruncateCollection_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil).Times(1)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(nil, errCollectionNotFound).Times(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats",
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.TruncateCollection(context.Background(), "testDB", "testCollection")
	require.Error(t, err, "Expected error when trying to truncate a non-existent collection")
	require.Equal(t, "collection not found", err.Error())
}

func Test_Client_ListCollections(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection1 := NewMockCollection(gomock.NewController(t))
	mockCollection2 := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collections(gomock.Any()).Return([]arangodb.Collection{mockCollection1, mockCollection2}, nil)
	mockCollection1.EXPECT().Name().Return("testCollection1")
	mockCollection2.EXPECT().Name().Return("testCollection2")
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	names, err := client.ListCollections(context.Background(), "testDB")
	require.NoError(t, err, "Expected no error while listing collections")
	require.Equal(t, []string{"testCollection1", "testCollection2"}, names)
}

func Test_Client_ListCollections_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collections(gomock.Any()).Return(nil, errCollectionNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Execute
	names, err := client.ListCollections(context.Background(), "testDB")
	require.Error(t, err, "collection not found")
	require.Nil(t, names)
}
