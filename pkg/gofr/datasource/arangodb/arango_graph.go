package arangodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

var (
	errInvalidInput        = errors.New("invalid input parameter")
	errInvalidResponseType = errors.New("invalid response type")
)

type EdgeDetails []arangodb.EdgeDetails

type Graph struct {
	client *Client
}

// GetEdges fetches all edges connected to a given vertex in the specified edge collection.
//
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - dbName: Database name.
//   - graphName: Graph name.
//   - edgeCollection: Edge collection name.
//   - vertexID: Full vertex ID (e.g., "persons/16563").
//   - resp: Pointer to `*EdgeDetails` to store results.
//
// Returns an error if input is invalid, `resp` is of the wrong type, or the query fails.
func (c *Client) GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string,
	resp any) error {
	if vertexID == "" || edgeCollection == "" {
		return errInvalidInput
	}

	// Type check the response parameter
	edgeResp, ok := resp.(*EdgeDetails)
	if !ok {
		return fmt.Errorf("%w: must be *[]arangodb.EdgeDetails", errInvalidResponseType)
	}

	tracerCtx, span := c.addTrace(ctx, "getEdges", map[string]string{
		"DB": dbName, "Graph": graphName, "Collection": edgeCollection, "Vertex": vertexID,
	})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Operation:  "getEdges",
		Database:   dbName,
		Collection: edgeCollection,
	}, startTime, "getEdges", span)

	db, err := c.client.Database(tracerCtx, dbName)
	if err != nil {
		return err
	}

	edges, err := db.GetEdges(tracerCtx, edgeCollection, vertexID, nil)
	if err != nil {
		return err
	}

	// Assign the result to the provided response parameter
	*edgeResp = edges

	return nil
}
