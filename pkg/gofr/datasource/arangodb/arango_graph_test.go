package arangodb

import (
	"context"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// setupGraphTest is a helper function to set up the common test environment for graph tests.
func setupGraphTest(t *testing.T) (*gomock.Controller, *MockArango, *MockDatabase, *MockLogger,
	*MockMetrics, *Client, *Graph, context.Context, string, string, *EdgeDefinition) {
	ctrl := gomock.NewController(t)

	mockArango := NewMockArango(ctrl)
	mockDB := NewMockDatabase(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	client := &Client{
		logger:  mockLogger,
		metrics: mockMetrics,
		client:  mockArango,
	}

	graph := &Graph{client: client}
	ctx := context.Background()
	databaseName := "testDB"
	graphName := "testGraph"
	edgeDefinitions := &EdgeDefinition{{Collection: "edgeColl", From: []string{"fromColl"}, To: []string{"toColl"}}}

	return ctrl, mockArango, mockDB, mockLogger, mockMetrics, client, graph, ctx, databaseName, graphName, edgeDefinitions
}

func TestGraph_CreateGraph_AlreadyExists(t *testing.T) {
	ctrl, mockArango, mockDB, mockLogger, mockMetrics, _, graph, ctx, databaseName, graphName, edgeDefinitions := setupGraphTest(t)
	defer ctrl.Finish()

	mockArango.EXPECT().Database(ctx, databaseName).Return(mockDB, nil)
	mockDB.EXPECT().GraphExists(ctx, graphName).Return(true, nil)
	mockLogger.EXPECT().Debugf("graph %s already exists in database %s", graphName, databaseName)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := graph.CreateGraph(ctx, databaseName, graphName, edgeDefinitions)
	require.Equal(t, ErrGraphExists, err, "Expected graph already exits error but got %v", err)
}

func TestGraph_CreateGraph_InvalidEdgeDocumentType(t *testing.T) {
	ctrl, mockArango, mockDB, mockLogger, mockMetrics, _, graph, ctx, databaseName, graphName, edgeDefinitions := setupGraphTest(t)
	defer ctrl.Finish()

	options := &arangodb.GraphDefinition{EdgeDefinitions: []arangodb.EdgeDefinition{{
		Collection: "edgeColl",
		From:       []string{"fromColl"},
		To:         []string{"toColl"},
	}}}

	mockArango.EXPECT().Database(ctx, databaseName).Return(mockDB, nil)
	mockDB.EXPECT().GraphExists(ctx, graphName).Return(false, nil)
	mockDB.EXPECT().CreateGraph(ctx, graphName, options, nil).Return(nil, errInvalidEdgeDocumentType)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := graph.CreateGraph(ctx, databaseName, graphName, edgeDefinitions)
	require.Equal(t, errInvalidEdgeDocumentType, err, "Expected err  %v but got %v",
		errInvalidEdgeDocumentType, err)
}
