# Elasticsearch Migrations

Elasticsearch migrations in **GoFr** let you manage index schemas, mappings, settings and data in a *version-controlled* manner.
This guide explains how to implement and operate these migrations without breaking production.

## Overview

Elasticsearch migrations help you:

- Create and manage indices with proper mappings
- Update index settings and configurations
- Seed initial data or migrate existing data
- Perform bulk operations efficiently
- Maintain schema consistency across environments


## Migration Tracking

GoFr automatically creates a `gofr_migrations` index in Elasticsearch to track applied migrations.
The index stores:

- Migration version (timestamp)
- Execution method (UP)
- Start time and duration
- Migration status


## Basic Migration Structure

```go
package main

import (
	"context"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/elasticsearch"
	"gofr.dev/pkg/gofr/migration"
)

func main() {
	app := gofr.New()

	// Configure Elasticsearch
	esClient := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	app.AddElasticsearch(esClient)

	// Define migrations
	migrationsMap := map[int64]migration.Migrate{
		1640995200: {
			UP: func(d migration.Datasource) error {
				// Migration logic here
				return nil
			},
		},
	}

	// Register and run migrations
	app.Migrate(migrationsMap)
	app.Run()
}
```


## Available Operations

### Index Management

```go
// Create an index with mappings and settings
CreateIndex(ctx context.Context, index string, settings map[string]any) error

// Delete an index
DeleteIndex(ctx context.Context, index string) error
```


### Document Operations

```go
// Index a single document
IndexDocument(ctx context.Context, index, id string, document any) error

// Delete a document by ID
DeleteDocument(ctx context.Context, index, id string) error

// Bulk operations for multiple documents
Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error)
```


## Migration Examples

### 1. Creating an Index with Mappings

```go
1640995200: {
	UP: func(d migration.Datasource) error {
		settings := map[string]any{
			"mappings": map[string]any{
				"properties": map[string]any{
					"title": map[string]any{
						"type":     "text",
						"analyzer": "standard",
					},
					"price": map[string]any{
						"type": "float",
					},
					"category": map[string]any{
						"type": "keyword",
					},
					"created_at": map[string]any{
						"type": "date",
					},
					"tags": map[string]any{
						"type": "keyword",
					},
				},
			},
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
				"analysis": map[string]any{
					"analyzer": map[string]any{
						"custom_text_analyzer": map[string]any{
							"type":      "standard",
							"stopwords": "_english_",
						},
					},
				},
			},
		}

		return d.Elasticsearch.CreateIndex(context.Background(), "products", settings)
	},
},
```


### 2. Seeding Initial Data

```go
1640995300: {
	UP: func(d migration.Datasource) error {
		// Create sample products
		products := []map[string]any{
			{
				"title":      "Laptop",
				"price":      999.99,
				"category":   "electronics",
				"created_at": "2024-01-01T00:00:00Z",
				"tags":       []string{"computer", "portable"},
			},
			{
				"title":      "Coffee Mug",
				"price":      12.99,
				"category":   "kitchen",
				"created_at": "2024-01-01T00:00:00Z",
				"tags":       []string{"ceramic", "drink"},
			},
		}

		ctx := context.Background()
		for i, product := range products {
			err := d.Elasticsearch.IndexDocument(
				ctx,
				"products",
				fmt.Sprintf("%d", i+1),
				product,
			)
			if err != nil {
				return fmt.Errorf("failed to index product %d: %w", i+1, err)
			}
		}

		return nil
	},
},
```


### 3. Bulk Operations Migration

```go
1640995400: {
	UP: func(d migration.Datasource) error {
		// Bulk index multiple documents efficiently
		operations := []map[string]any{
			// Index operation metadata
			{
				"index": map[string]any{
					"_index": "products",
					"_id":    "bulk_1",
				},
			},
			// Document data
			{
				"title":    "Bulk Product 1",
				"price":    19.99,
				"category": "bulk",
			},
			// Another index operation
			{
				"index": map[string]any{
					"_index": "products",
					"_id":    "bulk_2",
				},
			},
			// Document data
			{
				"title":    "Bulk Product 2",
				"price":    29.99,
				"category": "bulk",
			},
			// Delete operation
			{
				"delete": map[string]any{
					"_index": "products",
					"_id":    "old_product",
				},
			},
		}

		ctx := context.Background()
		result, err := d.Elasticsearch.Bulk(ctx, operations)
		if err != nil {
			return fmt.Errorf("bulk operation failed: %w", err)
		}

		// Check for errors in bulk response
		if errors, ok := result["errors"].(bool); ok && errors {
			return fmt.Errorf("bulk operation had errors: %v", result)
		}

		return nil
	},
},
```


### 4. Index Settings Update

```go
1640995500: {
	UP: func(d migration.Datasource) error {
		// Create a new index with updated settings
		settings := map[string]any{
			"mappings": map[string]any{
				"properties": map[string]any{
					"title": map[string]any{
						"type":     "text",
						"analyzer": "custom_text_analyzer",
					},
					"description": map[string]any{
						"type":     "text",
						"analyzer": "standard",
					},
					"price": map[string]any{
						"type": "float",
					},
				},
			},
			"settings": map[string]any{
				"number_of_shards":   2, // Increased shards
				"number_of_replicas": 1, // Added replica
				"refresh_interval":   "30s",
			},
		}

		return d.Elasticsearch.CreateIndex(context.Background(), "products_v2", settings)
	},
},
```


### 5. Data Migration Between Indices

```go
1640995600: {
	UP: func(d migration.Datasource) error {
		ctx := context.Background()

		// This would typically involve:
		// 1. Reading data from old index (using Search - not shown in interface yet)
		// 2. Transforming data if needed
		// 3. Bulk indexing to new index
		// 4. Deleting old index

		// For now, we'll create the new index structure
		newSettings := map[string]any{
			"mappings": map[string]any{
				"properties": map[string]any{
					"product_name": map[string]any{ // Renamed from 'title'
						"type": "text",
					},
					"product_price": map[string]any{ // Renamed from 'price'
						"type": "float",
					},
					"product_category": map[string]any{ // Renamed from 'category'
						"type": "keyword",
					},
				},
			},
		}

		err := d.Elasticsearch.CreateIndex(ctx, "products_new_schema", newSettings)
		if err != nil {
			return fmt.Errorf("failed to create new schema index: %w", err)
		}

		// Clean up old index
		return d.Elasticsearch.DeleteIndex(ctx, "products_old")
	},
},
```


## Bulk Operations Format

### Index Operation

```go
{
	"index": map[string]any{
		"_index": "index_name",
		"_id":    "document_id",
	},
}
// Followed by document data
{
	"field1": "value1",
	"field2": "value2",
}
```


### Update Operation

```go
{
	"update": map[string]any{
		"_index": "index_name",
		"_id":    "document_id",
	},
}
// Followed by update data
{
	"doc": map[string]any{
		"field1": "new_value1",
	},
}
```


### Delete Operation

```go
{
	"delete": map[string]any{
		"_index": "index_name",
		"_id":    "document_id",
	},
}
// No document data needed for delete
```


## Best Practices

### 1. Index Naming

- Use descriptive names: `users`, `products`, `orders`
- Consider versioning: `products_v1`, `products_v2`
- Use consistent naming conventions


### 2. Mapping Design

- Define explicit mappings rather than relying on dynamic mapping
- Choose appropriate field types
- Consider analyzer requirements for text fields
- Plan for future field additions


### 3. Settings Configuration

- Set appropriate shard and replica counts
- Configure refresh intervals based on use case
- Set up custom analyzers if needed


### 4. Migration Safety

- Test migrations on non-production data first
- Use bulk operations for large data sets
- Implement proper error handling
- Consider index aliases for zero-downtime migrations


### 5. Performance Considerations

- Use bulk operations for multiple documents
- Batch operations appropriately (1 000 – 5 000 docs per batch)
- Monitor cluster health during migrations
- Consider disabling replicas during large data migrations


## Error Handling

```go
UP: func(d migration.Datasource) error {
	ctx := context.Background()

	// Check if index already exists (idempotent migration)
	settings := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"name": map[string]any{"type": "text"},
			},
		},
	}

	err := d.Elasticsearch.CreateIndex(ctx, "users", settings)
	if err != nil {
		// Handle specific Elasticsearch errors
		if strings.Contains(err.Error(), "resource_already_exists_exception") {
			// Index already exists, this is okay
			return nil
		}
		return fmt.Errorf("failed to create users index: %w", err)
	}

	return nil
},
```


## Monitoring Migration Logs

```plaintext
INFO [15:09:13] running migration 1640995200
DEBU [15:09:13] CREATE INDEX products            ELASTIC   215759µs products             {"mappings":{"properties":{"price":{"type":"float"},"title":{"type":"text"}}},"settings":{"number_of_replicas":0,"number_of_shards":1}}
DEBU [15:09:13] INDEX DOCUMENT products/1        ELASTIC    87374µs 1                    {"price":19.99,"title":"Sample Product"}
```

The logs show:

- **Operation type** – CREATE INDEX, INDEX DOCUMENT, BULK, etc.
- **Execution time** – In microseconds
- **Target** – Index name, document ID
- **Query/Data** – Full JSON of the operation (no base64 encoding)


## Complete Example

```go
package main

import (
	"context"
	"fmt"
	"os"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/elasticsearch"
	"gofr.dev/pkg/gofr/migration"
)

func main() {
	app := gofr.New()

	// Configure Elasticsearch
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	esClient := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{esURL},
	})
	app.AddElasticsearch(esClient)

	// Define migrations
	migrationsMap := map[int64]migration.Migrate{
		// Create users index
		1640995200: {
			UP: func(d migration.Datasource) error {
				settings := map[string]any{
					"mappings": map[string]any{
						"properties": map[string]any{
							"name":  map[string]any{"type": "keyword"},
							"email": map[string]any{"type": "keyword"},
							"age":   map[string]any{"type": "integer"},
						},
					},
				}
				return d.Elasticsearch.CreateIndex(context.Background(), "users", settings)
			},
		},

		// Seed initial users
		1640995300: {
			UP: func(d migration.Datasource) error {
				users := []map[string]any{
					{"name": "Alice", "email": "alice@example.com", "age": 30},
					{"name": "Bob", "email": "bob@example.com", "age": 25},
				}

				ctx := context.Background()
				for i, user := range users {
					err := d.Elasticsearch.IndexDocument(
						ctx, "users", fmt.Sprintf("%d", i+1), user,
					)
					if err != nil {
						return err
					}
				}
				return nil
			},
		},

		// Bulk add more users
		1640995400: {
			UP: func(d migration.Datasource) error {
				operations := []map[string]any{
					{"index": map[string]any{"_index": "users", "_id": "3"}},
					{"name": "Carol", "email": "carol@example.com", "age": 28},
					{"index": map[string]any{"_index": "users", "_id": "4"}},
					{"name": "David", "email": "david@example.com", "age": 35},
				}

				_, err := d.Elasticsearch.Bulk(context.Background(), operations)
				return err
			},
		},
	}

	// Run migrations
	app.Migrate(migrationsMap)

	// Add API endpoints
	app.GET("/users", getUsersHandler)

	app.Run()
}

func getUsersHandler(ctx *gofr.Context) (any, error) {
	query := map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  10,
	}

	result, err := ctx.Container.Elasticsearch.Search(
		ctx.Context, []string{"users"}, query,
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}
```

**Enjoy consistent, version-controlled Elasticsearch migrations with GoFr!**

