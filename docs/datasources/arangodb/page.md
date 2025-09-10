# ArangoDB

GoFr supports injecting `ArangoDB` that implements the following interface. Any driver that implements the interface can be
added using the `app.AddArangoDB()` method, and users can use ArangoDB across the application with `gofr.Context`.

## Configuration

To connect to ArangoDB, you need to provide the following environment variables:
- **HOST**: The hostname or IP address of your ArangoDB server.
- **USER**: The username for connecting to the database.
- **PASSWORD**: The password for the specified user.
- **PORT**: The port number




```go
type ArangoDB interface {
    // CreateDB creates a new database in ArangoDB.
	CreateDB(ctx context.Context, database string) error
	// DropDB deletes an existing database in ArangoDB.
	DropDB(ctx context.Context, database string) error

	// CreateCollection creates a new collection in a database with specified type.
	CreateCollection(ctx context.Context, database, collection string, isEdge bool) error
	// DropCollection deletes an existing collection from a database.
	DropCollection(ctx context.Context, database, collection string) error

	// CreateGraph creates a new graph in a database.
	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error
	// DropGraph deletes an existing graph from a database.
	DropGraph(ctx context.Context, database, graph string) error

    // CreateDocument creates a new document in the specified collection.
	CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error)
	// GetDocument retrieves a document by its ID from the specified collection.
	GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error
	// UpdateDocument updates an existing document in the specified collection.
	UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error
	// DeleteDocument deletes a document by its ID from the specified collection.
	DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error

	// GetEdges retrieves all the edge documents connected to a specific vertex in an ArangoDB graph.
	GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string, resp any) error

	// Query executes an AQL query and binds the results
	Query(ctx context.Context, dbName string, query string, bindVars map[string]any, result any, options ...map[string]any) error

   HealthCheck(context.Context) (any, error)
}
```

Users can easily inject a driver that supports this interface, providing usability without compromising the extensibility to use multiple databases.

Import the GoFr's external driver for ArangoDB:

```shell
go get gofr.dev/pkg/gofr/datasource/arangodb@latest
```

### Example

```go
package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/arangodb"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	app := gofr.New()

	// Configure the ArangoDB client
	arangoClient := arangodb.New(arangodb.Config{
		Host:     "localhost",
		User:     "root",
		Password: "root",
		Port:     8529,
	})
	app.AddArangoDB(arangoClient)

	// Example routes demonstrating different types of operations
	app.POST("/setup", Setup)
	app.POST("/users/{name}", CreateUserHandler)
	app.POST("/friends", CreateFriendship)
	app.GET("/friends/{collection}/{vertexID}", GetEdgesHandler)

	app.Run()
}

// Setup demonstrates database and collection creation
func Setup(ctx *gofr.Context) (any, error) {
	_, err := ctx.ArangoDB.CreateDocument(ctx, "social_network", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	if err := createCollection(ctx, "social_network", "persons"); err != nil {
		return nil, err
	}
	if err := createCollection(ctx, "social_network", "friendships"); err != nil {
		return nil, err
	}

	// Define and create the graph
	edgeDefs := arangodb.EdgeDefinition{
		{Collection: "friendships", From: []string{"persons"}, To: []string{"persons"}},
	}

	_, err = ctx.ArangoDB.CreateDocument(ctx, "social_network", "social_graph", edgeDefs)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph: %w", err)
	}

	return "Setup completed successfully", nil
}

// Helper function to create collections
func createCollection(ctx *gofr.Context, dbName, collectionName string) error {
	_, err := ctx.ArangoDB.CreateDocument(ctx, dbName, collectionName, nil)
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", collectionName, err)
	}
	return nil
}

// CreateUserHandler demonstrates user management and document creation
func CreateUserHandler(ctx *gofr.Context) (any, error) {
	name := ctx.PathParam("name")

	// Create a person document
	person := Person{
		Name: name,
		Age:  25,
	}
	docID, err := ctx.ArangoDB.CreateDocument(ctx, "social_network", "persons", person)
	if err != nil {
		return nil, fmt.Errorf("failed to create person document: %w", err)
	}

	return map[string]string{
		"message": "User created successfully",
		"docID":   docID,
	}, nil
}

// CreateFriendship demonstrates edge document creation
func CreateFriendship(ctx *gofr.Context) (any, error) {
	var req struct {
		From      string `json:"from"`
		To        string `json:"to"`
		StartDate string `json:"startDate"`
	}

	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}

	edgeDocument := map[string]any{
		"_from":     fmt.Sprintf("persons/%s", req.From),
		"_to":       fmt.Sprintf("persons/%s", req.To),
		"startDate": req.StartDate,
	}

	// Create an edge document for the friendship
	edgeID, err := ctx.ArangoDB.CreateDocument(ctx, "social_network", "friendships", edgeDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to create friendship: %w", err)
	}

	return map[string]string{
		"message": "Friendship created successfully",
		"edgeID":  edgeID,
	}, nil
}

// GetEdgesHandler demonstrates fetching edges connected to a vertex
func GetEdgesHandler(ctx *gofr.Context) (any, error) {
	collection := ctx.PathParam("collection")
	vertexID := ctx.PathParam("vertexID")

	fullVertexID := fmt.Sprintf("%s/%s", collection, vertexID)

	// Prepare a slice to hold edge details
	edges := make(arangodb.EdgeDetails, 0)

	// Fetch all edges connected to the given vertex
	err := ctx.ArangoDB.GetEdges(ctx, "social_network", "social_graph", "friendships",
		fullVertexID, &edges)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}

	return map[string]any{
		"vertexID": vertexID,
		"edges":    edges,
	}, nil
}
```
