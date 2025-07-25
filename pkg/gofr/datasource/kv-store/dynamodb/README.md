# DynamoDB Client

This package provides a DynamoDB client for use as a key-value store in GoFr applications. 
It supports basic CRUD operations (Set, Get, Delete) on items using a partition key, along with health checks. 
The client integrates logging, metrics, and tracing for observability.

## Quick Start
Import the package and create the client instance:
```Go
import (
    "context"

    "gofr.dev/pkg/gofr/datasource/kv/dynamo" // Adjust path as needed
)

func main() {
    configs := dynamo.Configs{
        Table:            "your-table-name",
        Region:           "us-east-1",
        Endpoint:         "", // Leave empty for real AWS; set for local (e.g., "http://localhost:8000")
        PartitionKeyName: "pk", // Default is "pk" if not specified
    }

    client := dynamo.New(configs)
    client.UseLogger(yourLogger)   // Implement dynamo.Logger interface
    client.UseMetrics(yourMetrics) // Implement dynamo.Metrics interface
    // client.UseTracer(yourTracer) // Optional: trace.Tracer

    if err := client.Connect(); err != nil {
        // Handle connection error
    }

    ctx := context.Background()
    key := "example-key"
    attributes := map[string]any{"field": "value"}

    // Set item
    client.Set(ctx, key, attributes)

    // Get item
    result, _ := client.Get(ctx, key)

    // Delete item
    client.Delete(ctx, key)
}
```

## Configuration
The Configs struct defines the client settings:

* `Table`: Required. Name of the DynamoDB table.
* `Region`: Required. AWS region (e.g., "us-east-1").
* `Endpoint`: Optional. Custom endpoint URL (e.g., for local DynamoDB).
* `PartitionKeyName`: Optional. Partition key attribute name (defaults to "pk").

The table must have a string partition key (no sort key support).

## Usage

**Operations**
* `Set(ctx context.Context, key string, attributes map[string]any) error`: Stores attributes under the given key. Overwrites existing items.
* `Get(ctx context.Context, key string) (map[string]any, error)`: Retrieves attributes for the key. Returns error if key not found.
* `Delete(ctx context.Context, key string) error`: Deletes the item by key.
* `HealthCheck(ctx context.Context) (any, error)`: Checks table status via DescribeTable. Returns a `Health` struct with status ("UP" or "DOWN") and details.

All operations log details, record metrics (e.g., duration histograms), and support tracing spans.

## Observability
* Logging: Use `UseLogger` to inject a logger implementing the `Logger` interface. Logs details and errors.
* Metrics: Use `UseMetrics` to inject metrics implementing the `Metrics` interface. Records histograms for query durations.
* Tracing: Use `UseTracer` to inject an OpenTelemetry tracer. Adds spans for each operation.

## Notes
* This client assumes a simple key-value model; no support for sort keys, queries, or scans.
* For production, use IAM roles or credentials via AWS config.
* Ensure table exists before connecting; health check verifies accessibility.