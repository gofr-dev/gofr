package arangodb

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

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
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

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

	// Mock the database method to return a mock database instance
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
