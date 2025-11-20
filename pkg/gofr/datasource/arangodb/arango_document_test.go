package arangodb

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Client_CreateDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockCollection.EXPECT().Properties(gomock.Any()).Return(arangodb.CollectionProperties{}, nil)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockCollection.EXPECT().Properties(gomock.Any()).Return(arangodb.CollectionProperties{}, nil)
	mockCollection.EXPECT().CreateDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentCreateResponse{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	docName, err := client.CreateDocument(context.Background(), "testDB",
		"testCollection", "testDocument")
	require.Empty(t, docName)
	require.ErrorIs(t, err, errDocumentNotFound, "Expected error when document not found")
}

func Test_Client_GetDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockCollection.EXPECT().ReadDocument(gomock.Any(), "testDocument", "").
		Return(arangodb.DocumentMeta{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	err := client.GetDocument(context.Background(), "testDB",
		"testCollection", "testDocument", "")
	require.ErrorIs(t, err, errDocumentNotFound, "Expected error when document not found")
}

func Test_Client_UpdateDocument(t *testing.T) {
	client, mockArango, _, mockLogger, mockMetrics := setupDB(t)
	mockDB := NewMockDatabase(gomock.NewController(t))
	mockCollection := NewMockCollection(gomock.NewController(t))
	testDocument := map[string]any{"field": "value"}

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockCollection.EXPECT().UpdateDocument(gomock.Any(), "testDocument", testDocument).
		Return(arangodb.CollectionDocumentUpdateResponse{
			DocumentMetaWithOldRev: arangodb.DocumentMetaWithOldRev{DocumentMeta: arangodb.DocumentMeta{Key: "testKey", ID: "1", Rev: ""}}}, nil)
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
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

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDB, nil).AnyTimes()
	mockDB.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockCollection.EXPECT().DeleteDocument(gomock.Any(), "testDocument").
		Return(arangodb.CollectionDocumentDeleteResponse{}, errDocumentNotFound)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := client.DeleteDocument(context.Background(), "testDB", "testCollection",
		"testDocument")
	require.ErrorIs(t, err, errDocumentNotFound, "Expected error while updating the document")
}

func TestExecuteCollectionOperation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockClient(ctrl)
	mockDatabase := NewMockDatabase(ctrl)
	mockCollection := NewMockCollection(ctrl)

	client := New(Config{Host: "localhost", Port: 8527, User: "root", Password: "root"})
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.client = mockArango
	d := Document{client: client}

	ctx := context.Background()
	dbName := "testDB"
	collectionName := "testCollection"
	operation := "createDocument"
	documentID := "doc123"

	mockArango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).
		Return(mockDatabase, nil).AnyTimes()
	mockDatabase.EXPECT().GetCollection(gomock.Any(), "testCollection", nil).
		Return(mockCollection, nil).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(ctx, "app_arango_stats", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	_, _, err := executeCollectionOperation(ctx, d, dbName, collectionName, operation, documentID)
	require.NoError(t, err)
}

func TestValidateEdgeDocument(t *testing.T) {
	tests := []struct {
		name          string
		document      any
		expectedError error
	}{
		{
			name: "Success - Valid Edge Document",
			document: map[string]any{
				"_from": "vertex1",
				"_to":   "vertex2",
			},
			expectedError: nil,
		},
		{
			name:          "Fail - Document is Not a Map",
			document:      "invalid",
			expectedError: errInvalidEdgeDocumentType,
		},
		{
			name: "Fail - Missing _from Field",
			document: map[string]any{
				"_to": "vertex2",
			},
			expectedError: errMissingEdgeFields,
		},
		{
			name: "Fail - Missing _to Field",
			document: map[string]any{
				"_from": "vertex1",
			},
			expectedError: errMissingEdgeFields,
		},
		{
			name: "Fail - _from is Not a String",
			document: map[string]any{
				"_from": 123,
				"_to":   "vertex2",
			},
			expectedError: errInvalidFromField,
		},
		{
			name: "Fail - _to is Not a String",
			document: map[string]any{
				"_from": "vertex1",
				"_to":   123,
			},
			expectedError: errInvalidToField,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEdgeDocument(tc.document)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
