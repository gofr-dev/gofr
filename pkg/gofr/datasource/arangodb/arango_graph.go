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

type Graph struct {
	client *Client
}

// GetEdges retrieves all the edge documents connected to a specific vertex in an ArangoDB graph.
// The method performs a query on the specified edge collection to fetch all edges where the vertex
// is either the source (_from) or target (_to) of the edge. The results are returned and bound to
// the provided `resp` argument.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and timeouts.
//   - dbName: The name of the database in which the graph resides.
//   - graphName: The name of the graph to query edges from.
//   - edgeCollection: The name of the edge collection to query edges from.
//   - vertexID: The ID of the vertex whose connected edges are to be fetched.
//   - resp: A pointer to a slice of `[]arangodb.EdgeDetails` where the resulting edges will be stored.
//
// Returns:
//   - error: Returns an error if the query fails, the response type is invalid, or any of the input parameters are incorrect.
//
// Example usage:
//
//	var edges []arangodb.EdgeDetails
//	err := client.GetEdges(ctx, "testDB", "testGraph", "edges", "vertex123", &edges)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(edges)
func (c *Graph) GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string,
	resp any) error {
	if vertexID == "" || edgeCollection == "" {
		return errInvalidInput
	}

	_, ok := resp.(*[]arangodb.EdgeDetails)
	if !ok {
		return fmt.Errorf("%w: Must be *[]arangodb.EdgeDetails", errInvalidResponseType)
	}

	tracerCtx, span := c.client.addTrace(ctx, "getEdges", map[string]string{
		"DB": dbName, "Graph": graphName, "Collection": edgeCollection, "Vertex": vertexID,
	})
	startTime := time.Now()

	defer c.client.sendOperationStats(&QueryLog{
		Operation:  "getEdges",
		Database:   dbName,
		Collection: edgeCollection,
	}, startTime, "getEdges", span)

	// Define the query to get edges from the specified vertex
	query := `
		FOR edge IN @@edgeCollection
			FILTER edge._from == @vertexID OR edge._to == @vertexID
			RETURN edge
	`
	// Bind variables
	bindVars := map[string]any{
		"@edgeCollection": edgeCollection,
		"vertexID":        vertexID,
	}

	err := c.client.Query(tracerCtx, dbName, query, bindVars, resp)
	if err != nil {
		return err
	}

	return nil
}
