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

// arangoMocks bundles the mocks wired into a client built by newArangoTestClient.
type arangoMocks struct {
	arango     *MockClient
	db         *MockDatabase
	collection *MockCollection
	graph      *MockGraph
	logger     *MockLogger
}

// newArangoTestClient returns a fully initialized client (DB, Document and Graph
// handlers) backed by mocks, with logging and metrics expectations relaxed.
func newArangoTestClient(t *testing.T) (*Client, *arangoMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)

	m := &arangoMocks{
		arango:     NewMockClient(ctrl),
		db:         NewMockDatabase(ctrl),
		collection: NewMockCollection(ctrl),
		graph:      NewMockGraph(ctrl),
		logger:     NewMockLogger(ctrl),
	}

	mockMetrics := NewMockMetrics(ctrl)

	m.logger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := New(Config{Host: "localhost", Port: 8529, User: "root", Password: "root"})
	client.UseLogger(m.logger)
	client.UseMetrics(mockMetrics)
	client.client = m.arango

	return client, m
}

func Test_Client_CreateDocument_Cases(t *testing.T) {
	edgeDoc := map[string]any{"_from": "users/1", "_to": "orders/1"}
	edgeProps := arangodb.CollectionProperties{CollectionExtendedInfo: arangodb.CollectionExtendedInfo{
		CollectionInfo: arangodb.CollectionInfo{Type: arangoEdgeCollectionType}}}

	tests := []struct {
		desc       string
		document   any
		setupMocks func(m *arangoMocks)
		expID      string
		expErr     error
	}{
		{
			desc:     "database lookup fails",
			document: "doc",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(nil, errDBNotFound)
			},
			expErr: errDBNotFound,
		},
		{
			desc:     "edge check collection lookup fails",
			document: "doc",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil).Times(2)
				gomock.InOrder(
					m.db.EXPECT().GetCollection(gomock.Any(), "edges", nil).Return(m.collection, nil),
					m.db.EXPECT().GetCollection(gomock.Any(), "edges", nil).Return(nil, errCollectionNotFound),
				)
			},
			expErr: errCollectionNotFound,
		},
		{
			desc:     "collection properties fail",
			document: "doc",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil).Times(2)
				m.db.EXPECT().GetCollection(gomock.Any(), "edges", nil).Return(m.collection, nil).Times(2)
				m.collection.EXPECT().Properties(gomock.Any()).Return(arangodb.CollectionProperties{}, errStatusDown)
			},
			expErr: errStatusDown,
		},
		{
			desc:     "invalid edge document",
			document: "not a map",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil).Times(2)
				m.db.EXPECT().GetCollection(gomock.Any(), "edges", nil).Return(m.collection, nil).Times(2)
				m.collection.EXPECT().Properties(gomock.Any()).Return(edgeProps, nil)
			},
			expErr: errInvalidEdgeDocumentType,
		},
		{
			desc:     "valid edge document",
			document: edgeDoc,
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil).Times(2)
				m.db.EXPECT().GetCollection(gomock.Any(), "edges", nil).Return(m.collection, nil).Times(2)
				m.collection.EXPECT().Properties(gomock.Any()).Return(edgeProps, nil)
				m.collection.EXPECT().CreateDocument(gomock.Any(), edgeDoc).
					Return(arangodb.CollectionDocumentCreateResponse{DocumentMeta: arangodb.DocumentMeta{Key: "edge1"}}, nil)
			},
			expID: "edge1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, m := newArangoTestClient(t)
			tc.setupMocks(m)

			id, err := client.CreateDocument(t.Context(), "testDB", "edges", tc.document)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expID, id)
		})
	}
}

func Test_Client_DocumentOperations_DatabaseError(t *testing.T) {
	tests := []struct {
		desc string
		call func(ctx context.Context, c *Client) error
	}{
		{
			desc: "get document",
			call: func(ctx context.Context, c *Client) error { return c.GetDocument(ctx, "testDB", "coll", "id", nil) },
		},
		{
			desc: "update document",
			call: func(ctx context.Context, c *Client) error { return c.UpdateDocument(ctx, "testDB", "coll", "id", nil) },
		},
		{
			desc: "delete document",
			call: func(ctx context.Context, c *Client) error { return c.DeleteDocument(ctx, "testDB", "coll", "id") },
		},
		{
			desc: "drop collection",
			call: func(ctx context.Context, c *Client) error { return c.DropCollection(ctx, "testDB", "coll") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, m := newArangoTestClient(t)
			m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(nil, errDBNotFound)

			err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, errDBNotFound)
		})
	}
}
