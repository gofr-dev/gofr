package arangodb

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errCollectionNotFound = errors.New("collection not found")

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

func Test_Client_CreateDB_AlreadyExists(t *testing.T) {
	client, _, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "dbExists"

	mockLogger.EXPECT().Debugf("database %s already exists", database)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateDB(ctx, database)
	require.Equal(t, ErrDatabaseExists, err, "Expected error when database already exists")
}

func Test_Client_DropDB(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)

	ctx := context.Background()
	database := "testDB"
	mockDB := NewMockDatabase(gomock.NewController(t))

	// Mock the database method to return a mock database instance
	mockArango.EXPECT().GetDatabase(gomock.Any(), database, &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil).Times(1)
	mockDB.EXPECT().Remove(gomock.Any()).Return(nil).Times(1)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), database, &arangodb.GetDatabaseOptions{}).
		Return(nil, errDBNotFound).Times(1)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil).Times(1)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil)
	mockDB.EXPECT().CollectionExists(gomock.Any(), "testCollection").Return(false, nil)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil)
	mockDB.EXPECT().CollectionExists(gomock.Any(), "testCollection").Return(false, nil)
	mockDB.EXPECT().CreateCollection(gomock.Any(), "testCollection", gomock.Any()).Return(nil, errCollectionNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateCollection(context.Background(), "testDB", "testCollection", false)
	require.Error(t, err, "Expected an error while creating the collection")
	require.Equal(t, "collection not found", err.Error())
}

func Test_Client_CreateCollection_AlreadyExists(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))

	mockArango.EXPECT().GetDatabase(gomock.Any(), "dbExists", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil)
	mockDB.EXPECT().CollectionExists(gomock.Any(), "testCollection").Return(true, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf("collection %s already exists in database %s",
		"testCollection", "dbExists")
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.CreateCollection(context.Background(), "dbExists", "testCollection", true)
	require.Equal(t, ErrCollectionExists, err, "Expected error when collection already exists")
}

func Test_Client_DropCollection(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil)
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).Return(mockCollection, nil)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", &arangodb.GetDatabaseOptions{}).
		Return(mockDB, nil)
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(nil, errCollectionNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DropCollection(context.Background(), "testDB", "testCollection")
	require.Error(t, err, "Expected error when trying to drop a non-existent collection")
	require.Equal(t, "collection not found", err.Error())
}
