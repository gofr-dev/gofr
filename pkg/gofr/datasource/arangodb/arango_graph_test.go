package arangodb

import (
	"context"
	"github.com/arangodb/go-driver/v2/arangodb"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGraph_CreateGraph_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	mockArango.EXPECT().Database(ctx, databaseName).Return(mockDB, nil)
	mockDB.EXPECT().GraphExists(ctx, graphName).Return(true, nil)
	mockLogger.EXPECT().Debugf("graph %s already exists in database %s", graphName, databaseName)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := graph.CreateGraph(ctx, databaseName, graphName, edgeDefinitions)
	require.Equal(t, ErrGraphExists, err, "Expected graph already exits error but got %v", err)
}

func TestGraph_CreateGraph_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
	options := &arangodb.GraphDefinition{EdgeDefinitions: []arangodb.EdgeDefinition{{
		Collection: "edgeColl",
		To:         []string{"toColl"},
		From:       []string{"fromColl"},
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
