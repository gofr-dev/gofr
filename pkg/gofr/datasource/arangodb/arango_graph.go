package arangodb

import (
	"context"
	"errors"
	"fmt"

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
// It first checks if the graph already exists before attempting to create it.
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - database: Name of the database where the graph will be created.
//   - graph: Name of the graph to be created.
//   - edgeDefinitions: Pointer to EdgeDefinition struct containing edge definitions.
//
// Returns ErrGraphExists if the graph already exists.
// Returns an error if the edgeDefinitions parameter is not of type *EdgeDefinition or is nil.
func (g *Graph) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	ctx, done := g.client.instrumentOp(ctx, &QueryLog{Operation: "createGraph",
		Database: database, Graph: graph})
	defer done()

	db, err := g.client.client.GetDatabase(ctx, database, nil)
	if err != nil {
		return err
	}

	// Check if the graph already exists
	exists, err := db.GraphExists(ctx, graph)
	if err != nil {
		return err
	}

	if exists {
		g.client.logger.Debugf("graph %s already exists in database %s", graph, database)
		return ErrGraphExists
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

	_, err = db.CreateGraph(ctx, graph, options, nil)

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
	ctx, done := g.client.instrumentOp(ctx, &QueryLog{Operation: "dropGraph",
		Database: database, Graph: graphName})
	defer done()

	db, err := g.client.client.GetDatabase(ctx, database, nil)
	if err != nil {
		return err
	}

	graph, err := db.Graph(ctx, graphName, nil)
	if err != nil {
		return err
	}

	err = graph.Remove(ctx, &arangodb.RemoveGraphOptions{DropCollections: true})
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

	ctx, done := c.instrumentOp(ctx, &QueryLog{
		Operation:  "getEdges",
		Database:   dbName,
		Graph:      graphName,
		Collection: edgeCollection,
	})
	defer done()

	db, err := c.client.GetDatabase(ctx, dbName, nil)
	if err != nil {
		return err
	}

	edges, err := db.GetEdges(ctx, edgeCollection, vertexID, nil)
	if err != nil {
		return err
	}

	// Assign the result to the provided response parameter
	*edgeResp = edges

	return nil
}
