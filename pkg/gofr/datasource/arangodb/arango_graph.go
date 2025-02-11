package arangodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

var (
	errInvalidEdgeDefinitionsType = errors.New("edgeDefinitions must be a *EdgeDefinition type")
	errNilEdgeDefinitions         = errors.New("edgeDefinitions cannot be nil")
	errInvalidInput               = errors.New("invalid input parameter")
	errInvalidResponseType        = errors.New("invalid response type")
)

type EdgeDetails []arangodb.EdgeDetails

type Graph struct {
	client *Client
}

// CreateGraph creates a new graph in a database.
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - database: Name of the database where the graph will be created.
//   - graph: Name of the graph to be created.
//   - edgeDefinitions: Pointer to EdgeDefinition struct containing edge definitions.
//
// Returns an error if the edgeDefinitions parameter is not of type *EdgeDefinition or is nil.
func (g *Graph) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	tracerCtx, span := g.client.addTrace(ctx, "createGraph", map[string]string{"graph": graph})
	startTime := time.Now()

	defer g.client.sendOperationStats(&QueryLog{Operation: "createGraph",
		Database: database, Collection: graph}, startTime, "createGraph", span)

	db, err := g.client.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	edgeDefs, ok := edgeDefinitions.(*EdgeDefinition)
	if !ok {
		return fmt.Errorf("%w", errInvalidEdgeDefinitionsType)
	}

	if edgeDefs == nil {
		return fmt.Errorf("%w", errNilEdgeDefinitions)
	}

	arangoEdgeDefs := make(EdgeDefinition, 0, len(*edgeDefs))
	for _, ed := range *edgeDefs {
		arangoEdgeDefs = append(arangoEdgeDefs, arangodb.EdgeDefinition{
			Collection: ed.Collection,
			From:       ed.From,
			To:         ed.To,
		})
	}

	options := &arangodb.GraphDefinition{
		EdgeDefinitions: arangoEdgeDefs,
	}

	_, err = db.CreateGraph(tracerCtx, graph, options, nil)

	return err
}

// DropGraph deletes an existing graph from a database.
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - database: Name of the database where the graph exists.
//   - graphName: Name of the graph to be deleted.
//
// Returns an error if the graph does not exist or if there is an issue with the database connection.
func (g *Graph) DropGraph(ctx context.Context, database, graphName string) error {
	tracerCtx, span := g.client.addTrace(ctx, "dropGraph", map[string]string{"graph": graphName})
	startTime := time.Now()

	defer g.client.sendOperationStats(&QueryLog{Operation: "dropGraph",
		Database: database}, startTime, "dropGraph", span)

	db, err := g.client.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	graph, err := db.Graph(tracerCtx, graphName, nil)
	if err != nil {
		return err
	}

	err = graph.Remove(tracerCtx, &arangodb.RemoveGraphOptions{DropCollections: true})
	if err != nil {
		return err
	}

	return err
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
