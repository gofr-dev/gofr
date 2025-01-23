package arango

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Client_CreateDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().CreateDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentCreateResponse{DocumentMeta: arangodb.DocumentMeta{
			Key: "testDocument", ID: "1"}}, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	docName, err := client.CreateDocument(context.Background(), "testDB",
		"testCollection", "testDocument")
	require.Equal(t, "testDocument", docName)
	require.NoError(t, err, "Expected no error while truncating the collection")
}

func Test_Client_CreateDocument_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().CreateDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentCreateResponse{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	docName, err := client.CreateDocument(context.Background(), "testDB",
		"testCollection", "testDocument")
	require.Equal(t, "", docName)
	require.ErrorIs(t, err, errDocumentNotFound, err, "Expected error when document not found")
}

func Test_Client_GetDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().ReadDocument(gomock.Any(), "testDocument", "").Return(arangodb.DocumentMeta{
		Key: "testKey", ID: "1"}, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.GetDocument(context.Background(), "testDB",
		"testCollection", "testDocument", "")
	require.NoError(t, err, "Expected no error while reading  the document")
}

func Test_Client_GetDocument_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().ReadDocument(gomock.Any(), "testDocument", "").
		Return(arangodb.DocumentMeta{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	err := client.GetDocument(context.Background(), "testDB",
		"testCollection", "testDocument", "")
	require.ErrorIs(t, err, errDocumentNotFound, err, "Expected error when document not found")
}

func Test_Client_UpdateDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))
	testDocument := map[string]any{"field": "value"}

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().UpdateDocument(gomock.Any(), "testDocument", testDocument).
		Return(arangodb.CollectionDocumentUpdateResponse{
			DocumentMeta: arangodb.DocumentMeta{Key: "testKey", ID: "1", Rev: ""}}, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.UpdateDocument(context.Background(), "testDB", "testCollection",
		"testDocument", testDocument)
	require.NoError(t, err, "Expected no error while updating the document")
}

func Test_Client_UpdateDocument_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))
	testDocument := map[string]any{"field": "value"}

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().UpdateDocument(gomock.Any(), "testDocument", testDocument).
		Return(arangodb.CollectionDocumentUpdateResponse{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.UpdateDocument(context.Background(), "testDB", "testCollection", "testDocument", testDocument)
	require.ErrorIs(t, err, errDocumentNotFound, "Expected error while updating the document")
}

func Test_Client_DeleteDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().DeleteDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentDeleteResponse{
			DocumentMeta: arangodb.DocumentMeta{Key: "testKey", ID: "1", Rev: ""}}, nil)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DeleteDocument(context.Background(), "testDB", "testCollection",
		"testDocument")
	require.NoError(t, err, "Expected no error while updating the document")
}

func Test_Client_DeleteDocument_Error(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().Database(gomock.Any(), "testDB").Return(mockDB, nil)
	mockDB.EXPECT().Collection(gomock.Any(), "testCollection").Return(mockCollection, nil)
	mockCollection.EXPECT().DeleteDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentDeleteResponse{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DeleteDocument(context.Background(), "testDB", "testCollection",
		"testDocument")
	require.ErrorIs(t, err, errDocumentNotFound, "Expected error while updating the document")
}
