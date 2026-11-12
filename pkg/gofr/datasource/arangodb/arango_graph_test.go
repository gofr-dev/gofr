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

func TestGraph_Operations_Errors(t *testing.T) {
	var edges EdgeDetails

	edgeDefs := &EdgeDefinition{{Collection: "edgeColl", From: []string{"fromColl"}, To: []string{"toColl"}}}

	tests := []struct {
		desc       string
		setupMocks func(m *arangoMocks)
		call       func(ctx context.Context, c *Client) error
		expErr     error
	}{
		{
			desc: "create graph database lookup fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(nil, errDBNotFound)
			},
			call:   func(ctx context.Context, c *Client) error { return c.CreateGraph(ctx, "testDB", "g", edgeDefs) },
			expErr: errDBNotFound,
		},
		{
			desc: "create graph existence check fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().GraphExists(gomock.Any(), "g").Return(false, errStatusDown)
			},
			call:   func(ctx context.Context, c *Client) error { return c.CreateGraph(ctx, "testDB", "g", edgeDefs) },
			expErr: errStatusDown,
		},
		{
			desc: "create graph with invalid edge definitions type",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().GraphExists(gomock.Any(), "g").Return(false, nil)
			},
			call:   func(ctx context.Context, c *Client) error { return c.CreateGraph(ctx, "testDB", "g", "invalid") },
			expErr: errInvalidEdgeDefinitionsType,
		},
		{
			desc: "create graph with nil edge definitions",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().GraphExists(gomock.Any(), "g").Return(false, nil)
			},
			call: func(ctx context.Context, c *Client) error {
				return c.CreateGraph(ctx, "testDB", "g", (*EdgeDefinition)(nil))
			},
			expErr: errNilEdgeDefinitions,
		},
		{
			desc: "drop graph lookup fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().Graph(gomock.Any(), "g", nil).Return(nil, errStatusDown)
			},
			call:   func(ctx context.Context, c *Client) error { return c.DropGraph(ctx, "testDB", "g") },
			expErr: errStatusDown,
		},
		{
			desc: "drop graph remove fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().Graph(gomock.Any(), "g", nil).Return(m.graph, nil)
				m.graph.EXPECT().Remove(gomock.Any(), &arangodb.RemoveGraphOptions{DropCollections: true}).Return(errStatusDown)
			},
			call:   func(ctx context.Context, c *Client) error { return c.DropGraph(ctx, "testDB", "g") },
			expErr: errStatusDown,
		},
		{
			desc: "get edges query fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().GetEdges(gomock.Any(), "edgeColl", "v/1", nil).Return(nil, errStatusDown)
			},
			call: func(ctx context.Context, c *Client) error {
				return c.GetEdges(ctx, "testDB", "g", "edgeColl", "v/1", &edges)
			},
			expErr: errStatusDown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, m := newArangoTestClient(t)
			tc.setupMocks(m)

			err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}
