package arango

import (
	"context"
	"errors"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/arangodb/shared"
)

type Graph struct {
	client *Client
}

// CreateGraph creates a new graph in a database.
func (g *Graph) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions []EdgeDefinition) error {
	tracerCtx, span := g.client.addTrace(ctx, "createGraph", map[string]string{"graph": graph})
	startTime := time.Now()

	defer g.client.sendOperationStats(&QueryLog{Query: "createGraph", Collection: graph}, startTime, "createGraph", span)

	db, err := g.client.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	arangoEdgeDefs := make([]arangodb.EdgeDefinition, 0, len(edgeDefinitions))
	for _, ed := range edgeDefinitions {
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
func (g *Graph) DropGraph(ctx context.Context, database, graphName string) error {
	tracerCtx, span := g.client.addTrace(ctx, "dropGraph", map[string]string{"graph": graphName})
	startTime := time.Now()

	defer g.client.sendOperationStats(&QueryLog{Query: "dropGraph", Collection: graphName}, startTime, "dropGraph", span)

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

// ListGraphs lists all graphs in a database.
func (g *Graph) ListGraphs(ctx context.Context, database string) ([]string, error) {
	tracerCtx, span := g.client.addTrace(ctx, "listGraphs", map[string]string{})
	startTime := time.Now()

	defer g.client.sendOperationStats(&QueryLog{Query: "listGraphs", Collection: database}, startTime, "listGraphs", span)

	db, err := g.client.client.Database(tracerCtx, database)
	if err != nil {
		return nil, err
	}

	graphsReader, err := db.Graphs(tracerCtx)
	if err != nil {
		return nil, err
	}

	var graphNames []string

	for {
		graph, err := graphsReader.Read()
		if errors.Is(err, shared.NoMoreDocumentsError{}) {
			break
		}

		if err != nil {
			return nil, err
		}

		graphNames = append(graphNames, graph.Name())
	}

	return graphNames, nil
}
