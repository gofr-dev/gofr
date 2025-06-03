package pinecone

import (
	"context"
)

// Config represents the configuration for connecting to Pinecone using the official SDK.
type Config struct {
	APIKey string // API key for authentication (required)
}

// Logger interface defines methods for logging operations with Pinecone.
type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
}

// Pinecone defines the methods for interacting with Pinecone vector database.
type Pinecone interface {
	Connect()
	UseLogger(logger any)
	UseMetrics(metrics any)
	UseTracer(tracer any)

	// ListIndexes returns all available indexes in the Pinecone project
	ListIndexes(ctx context.Context) ([]string, error)

	// DescribeIndex retrieves detailed information about a specific index
	DescribeIndex(ctx context.Context, indexName string) (map[string]any, error)

	// CreateIndex creates a new Pinecone index with the given parameters
	CreateIndex(ctx context.Context, indexName string, dimension int, metric string, options map[string]any) error

	// DeleteIndex deletes a Pinecone index
	DeleteIndex(ctx context.Context, indexName string) error

	// Upsert adds or updates vectors in a specific namespace of an index
	Upsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error)

	// Query searches for similar vectors in the index
	Query(ctx context.Context, indexName, namespace string, vector []float32, topK int, includeValues bool, includeMetadata bool, filter map[string]any) ([]any, error)

	// Fetch retrieves vectors by their IDs
	Fetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error)

	// Delete removes vectors from the index
	Delete(ctx context.Context, indexName, namespace string, ids []string) error

	// DeleteAll removes all vectors from a namespace
	DeleteAll(ctx context.Context, indexName, namespace string) error

	// HealthCheck performs a health check on the Pinecone connection
	HealthCheck(ctx context.Context) (any, error)
}

// Vector represents a vector in Pinecone
type Vector struct {
	ID         string            `json:"id"`
	Values     []float32         `json:"values"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	SparseData *SparseVectorData `json:"sparseValues,omitempty"`
}

// SparseVectorData represents sparse vector data
type SparseVectorData struct {
	Indices []int32   `json:"indices"`
	Values  []float32 `json:"values"`
}

// ScoredVector represents a vector with a similarity score returned from a query
type ScoredVector struct {
	ID         string            `json:"id"`
	Score      float32           `json:"score"`
	Values     []float32         `json:"values,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	SparseData *SparseVectorData `json:"sparseValues,omitempty"`
}
