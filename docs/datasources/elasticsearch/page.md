# Elasticsearch

## Configuration
To connect to `Elasticsearch`, you need to provide the following environment variables:
- `ADDRESSES`: Set of elasticsearch node URLs that the client will connect to.
- `USERNAME`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.

## Setup

GoFr supports injecting Elasticsearch with an interface that defines the 
necessary methods for interacting with Elasticsearch. 
Any driver that implements the following interface can be added using 
the app.AddElasticsearch() method.

```go
// Elasticsearch defines the methods for interacting with an Elasticsearch database.
type Elasticsearch interface {
    // Connect initializes the Elasticsearch client with the provided configuration.
    Connect()
    
    // CreateIndex creates an index with specified settings.
    CreateIndex(ctx context.Context, index string, settings map[string]any) error
    
    // DeleteIndex removes an index from Elasticsearch.
    DeleteIndex(ctx context.Context, index string) error
    
    // IndexDocument creates or replaces a document in the specified index.
    IndexDocument(ctx context.Context, index, id string, document any) error
    
    // GetDocument retrieves a document by its ID.
    GetDocument(ctx context.Context, index, id string) (map[string]any, error)
    
    // UpdateDocument applies a partial update to an existing document.
    UpdateDocument(ctx context.Context, index, id string, update map[string]any) error
    
    // DeleteDocument removes a document from an index.
    DeleteDocument(ctx context.Context, index, id string) error
    
    // Search executes a search query against one or more indices.
    Search(ctx context.Context, indices []string, query map[string]any) (map[string]any, error)
    
    // Bulk executes multiple operations in a single API call.
    Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error)
    
    // HealthCheck verifies connectivity to the Elasticsearch cluster.
    HealthChecker
}
```

Users can easily inject a driver that supports this interface, allowing for flexibility
without compromising usability. This structure supports all common Elasticsearch 
operations including indexing, searching, and document management.

Import the gofr's external driver for Elasticsearch:

```shell
go get gofr.dev/pkg/gofr/datasource/elasticsearch@latest
```

### Example

```go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/elasticsearch"
)

func main() {
	// Create a new application
	app := gofr.New()

	// Create Elasticsearch client with configuration
	es := elasticsearch.New(elasticsearch.Config{
		Addresses: app.Config.Get("ADDRESSES"),
			Username:  app.Config.Get("USERNAME"),
		Password:  app.Config.Get("PASSWORD"),
	})

	// Add Elasticsearch to the application
	app.AddElasticsearch(es)

	// Add routes for Elasticsearch operations
	app.POST("/documents", CreateDocumentHandler)
	app.GET("/documents/{id}", GetDocumentHandler)
	app.GET("/search", SearchDocumentsHandler)

	// Run the application
	app.Run()
}

// CreateDocumentHandler handles POST requests to create documents in Elasticsearch
func CreateDocumentHandler(c *gofr.Context) (any, error) {
	// Parse request body
	var document map[string]any
	if err := json.NewDecoder(c.Request().Body).Decode(&document); err != nil {
		return nil, err
	}

	// Get document ID from request or generate one
	id := c.Param("id")
	if id == "" {
		id = c.Header("X-Document-ID")
	}

	// Index the document in Elasticsearch
	err := c.Elasticsearch.IndexDocument(c, "products", id, document)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "document created", "id": id}, nil
}

// GetDocumentHandler handles GET requests to retrieve documents from Elasticsearch
func GetDocumentHandler(c *gofr.Context) (any, error) {
	// Get document ID from URL parameter
	id := c.PathParam("id")
	if id == "" {
		return nil, gofr.NewError(http.StatusBadRequest, "document ID is required")
	}

	// Retrieve the document from Elasticsearch
	result, err := c.Elasticsearch.GetDocument(c, "products", id)
	if err != nil {
		return nil, err
	}

	return result["_source"], nil
}

// SearchDocumentsHandler handles GET requests to search documents in Elasticsearch
func SearchDocumentsHandler(c *gofr.Context) (any, error) {
	query := c.Param("q")
	
	// Build search query
	searchQuery := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  query,
				"fields": []string{"name", "description"},
			},
		},
	}

	// Execute search
	result, err := c.Elasticsearch.Search(c, []string{"products"}, searchQuery)
	if err != nil {
		return nil, err
	}

	// Process and return search hits
	hits := result["hits"].(map[string]any)["hits"].([]any)
	documents := make([]map[string]any, len(hits))

	for i, hit := range hits {
		hitMap := hit.(map[string]any)
		documents[i] = hitMap["_source"].(map[string]any)
		documents[i]["id"] = hitMap["_id"]
	}

	return documents, nil
}
```
