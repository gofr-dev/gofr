package arangodb

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestGraph represents the test environment for graph-related tests.
type TestGraph struct {
	Ctrl        *gomock.Controller
	MockArango  *MockClient
	MockDB      *MockDatabase
	MockLogger  *MockLogger
	MockMetrics *MockMetrics
	Client      *Client
	Graph       *Graph
	Ctx         context.Context
	DBName      string
	GraphName   string
	EdgeDefs    *EdgeDefinition
}

// setupGraphTest creates a new test environment for graph tests.
func setupGraphTest(t *testing.T) *TestGraph {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockArango := NewMockClient(ctrl)
	mockDB := NewMockDatabase(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Setup common expectations
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := &Client{
		logger:  mockLogger,
		metrics: mockMetrics,
		client:  mockArango,
	}

	graph := &Graph{client: client}
	ctx := context.Background()

	return &TestGraph{
		Ctrl:        ctrl,
		MockArango:  mockArango,
		MockDB:      mockDB,
		MockLogger:  mockLogger,
		MockMetrics: mockMetrics,
		Client:      client,
		Graph:       graph,
		Ctx:         ctx,
		DBName:      "testDB",
		GraphName:   "testGraph",
		EdgeDefs:    &EdgeDefinition{{Collection: "edgeColl", From: []string{"fromColl"}, To: []string{"toColl"}}},
	}
}

func TestGraph_CreateGraph_Success(t *testing.T) {
	// Setup
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	mockGraph := NewMockGraph(test.Ctrl)
	graphInterface := arangodb.Graph(mockGraph)

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().GraphExists(test.Ctx, test.GraphName).Return(false, nil)
	test.MockDB.EXPECT().CreateGraph(
		test.Ctx, test.GraphName, gomock.Any(), nil,
	).Return(graphInterface, nil)

	err := test.Graph.CreateGraph(test.Ctx, test.DBName, test.GraphName, test.EdgeDefs)

	require.NoError(t, err, "expected err to be nil but got %v", err)
}

func TestGraph_CreateGraph_AlreadyExists(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().GraphExists(test.Ctx, test.GraphName).Return(true, nil)
	test.MockLogger.EXPECT().Debugf("graph %s already exists in database %s", test.GraphName, test.DBName)

	err := test.Graph.CreateGraph(test.Ctx, test.DBName, test.GraphName, test.EdgeDefs)

	assert.Equal(t, ErrGraphExists, err, "Expected graph already exits error but got %v", err)
}

func TestGraph_CreateGraph_Error(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	options := &arangodb.GraphDefinition{EdgeDefinitions: []arangodb.EdgeDefinition{{
		Collection: "edgeColl",
		From:       []string{"fromColl"},
		To:         []string{"toColl"},
	}}}

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().GraphExists(test.Ctx, test.GraphName).Return(false, nil)
	test.MockDB.EXPECT().CreateGraph(test.Ctx, test.GraphName, options, nil).Return(nil, errInvalidEdgeDocumentType)

	err := test.Graph.CreateGraph(test.Ctx, test.DBName, test.GraphName, test.EdgeDefs)

	assert.Equal(t, errInvalidEdgeDocumentType, err, "Expected err %v but got %v",
		errInvalidEdgeDocumentType, err)
}

func TestGraph_DropGraph_Success(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	mockGraph := NewMockGraph(test.Ctrl)
	graphInterface := arangodb.Graph(mockGraph)

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().Graph(test.Ctx, test.GraphName, nil).Return(graphInterface, nil)
	mockGraph.EXPECT().Remove(test.Ctx, &arangodb.RemoveGraphOptions{DropCollections: true}).Return(nil)

	err := test.Graph.DropGraph(test.Ctx, test.DBName, test.GraphName)

	require.NoError(t, err, "expected err to be nil but got %v", err)
}

func TestGraph_DropGraph_DBError(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(nil, errDBNotFound)

	err := test.Graph.DropGraph(test.Ctx, test.DBName, test.GraphName)

	assert.Equal(t, errDBNotFound, err, "expected err %v but got %v", errDBNotFound, err)
}

func TestGraph_DropGraph_Error(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	mockGraph := NewMockGraph(test.Ctrl)
	graphInterface := arangodb.Graph(mockGraph)

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().Graph(test.Ctx, test.GraphName, nil).Return(graphInterface, nil)
	mockGraph.EXPECT().Remove(test.Ctx, &arangodb.RemoveGraphOptions{DropCollections: true}).Return(errStatusDown)

	err := test.Graph.DropGraph(test.Ctx, test.DBName, test.GraphName)

	assert.Equal(t, errStatusDown, err, "expected err %v but got %v", errStatusDown, err)
}

func TestClient_GetEdges_Success(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	edgeCollection := "edgeColl"
	vertexID := "vertexID"

	expectedEdges := []arangodb.EdgeDetails{{
		To:    "toColl",
		From:  "fromColl",
		Label: "label",
	}}

	var resp EdgeDetails

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().GetEdges(test.Ctx, edgeCollection, vertexID, nil).Return(expectedEdges, nil)

	err := test.Client.GetEdges(test.Ctx, test.DBName, test.GraphName, edgeCollection, vertexID, &resp)

	require.NoError(t, err)
	assert.Equal(t, expectedEdges, []arangodb.EdgeDetails(resp))
}

func TestClient_GetEdges_DBError(t *testing.T) {
	// Setup
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	edgeCollection := "edgeColl"
	vertexID := "vertexID"

	var resp EdgeDetails

	test.MockArango.EXPECT().GetDatabase(test.Ctx, test.DBName, nil).
		Return(nil, errDBNotFound)

	err := test.Client.GetEdges(test.Ctx, test.DBName, test.GraphName, edgeCollection, vertexID, &resp)

	require.Error(t, err)
	require.Equal(t, errDBNotFound, err)
}

func TestClient_GetEdges_InvalidInput(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	var resp EdgeDetails

	err := test.Client.GetEdges(test.Ctx, test.DBName, test.GraphName, "", "", &resp)

	require.Error(t, err)
	require.Equal(t, errInvalidInput, err)
}

func TestClient_GetEdges_InvalidResponseType(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	edgeCollection := "edgeColl"
	vertexID := "vertexID"

	var resp string

	err := test.Client.GetEdges(test.Ctx, test.DBName, test.GraphName, edgeCollection, vertexID, &resp)

	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidResponseType)
}
