## DynamoDB

GoFr supports injecting DynamoDB as a key-value store that implements the standard KVStore interface. Any driver that implements the interface can be added
using `app.AddKVStore()` method, and users can use DynamoDB across application with `gofr.Context`.

```go
type KVStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error

	HealthChecker
}
```

Users can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.

Import the gofr's external driver for DynamoDB:

```shell
go get gofr.dev/pkg/gofr/datasource/kv-store/dynamodb@latest
```

### Example

```go
package main

import (
	"encoding/json"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/kv-store/dynamodb"
)

func main() {
	app := gofr.New()

	// Create DynamoDB client with configuration
	db := dynamodb.New(dynamodb.Configs{
		Table:            "my-table",
		Region:           "us-east-1",
		Endpoint:         "", // Leave empty for real AWS; set for local DynamoDB
		PartitionKeyName: "pk", // Default is "pk" if not specified
	})

	// inject the DynamoDB into gofr to use DynamoDB across the application
	// using gofr context
	app.AddKVStore(db)

	app.POST("/dynamodb", SetData)
	app.GET("/dynamodb/{key}", GetData)
	app.DELETE("/dynamodb/{key}", DeleteData)
	app.GET("/health", HealthCheck)

	app.Run()
}

func SetData(ctx *gofr.Context) (any, error) {
	key := ctx.Param("key")
	if key == "" {
		key = "default-key"
	}

	// Create a JSON string value
	value := map[string]any{
		"name":      "John Doe",
		"email":     "john@example.com",
		"created":   time.Now().Unix(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonValue, err := json.Marshal(value)
	if err != nil {
		ctx.Logger.Errorf("Error marshaling data: %v", err)
		return nil, err
	}

	err = ctx.KVStore.Set(ctx, key, string(jsonValue))
	if err != nil {
		ctx.Logger.Errorf("Error setting data: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Successfully stored data for key: %s", key)
	return map[string]string{"status": "data stored", "key": key}, nil
}

func GetData(ctx *gofr.Context) (any, error) {
	key := ctx.PathParam("key")
	if key == "" {
		return nil, gofr.NewError(400, "key parameter is required")
	}

	result, err := ctx.KVStore.Get(ctx, key)
	if err != nil {
		ctx.Logger.Errorf("Error getting data for key %s: %v", key, err)
		return nil, err
	}

	// Parse the JSON string back to a map
	var data map[string]any
	err = json.Unmarshal([]byte(result), &data)
	if err != nil {
		ctx.Logger.Errorf("Error unmarshaling data for key %s: %v", key, err)
		return nil, err
	}

	ctx.Logger.Infof("Successfully retrieved data for key: %s", key)
	return data, nil
}

func DeleteData(ctx *gofr.Context) (any, error) {
	key := ctx.PathParam("key")
	if key == "" {
		return nil, gofr.NewError(400, "key parameter is required")
	}

	err := ctx.KVStore.Delete(ctx, key)
	if err != nil {
		ctx.Logger.Errorf("Error deleting data for key %s: %v", key, err)
		return nil, err
	}

	ctx.Logger.Infof("Successfully deleted data for key: %s", key)
	return map[string]string{"status": "data deleted", "key": key}, nil
}

func HealthCheck(ctx *gofr.Context) (any, error) {
	health, err := ctx.KVStore.HealthCheck(ctx)
	if err != nil {
		ctx.Logger.Errorf("DynamoDB health check failed: %v", err)
		return map[string]string{"status": "DOWN", "error": err.Error()}, nil
	}

	ctx.Logger.Infof("DynamoDB health check passed: %+v", health)
	return health, nil
}
```

### Configuration

The DynamoDB client supports the following configuration options:

- `Table`: Required. Name of the DynamoDB table.
- `Region`: Required. AWS region (e.g., "us-east-1").
- `Endpoint`: Optional. Custom endpoint URL (e.g., for local DynamoDB).
- `PartitionKeyName`: Optional. Partition key attribute name (defaults to "pk").

### Local Development

For local development, you can use DynamoDB Local:

```bash
docker run --name dynamodb-local -d -p 8000:8000 amazon/dynamodb-local
```

Then configure your application to use the local endpoint:

```go
db := dynamodb.New(dynamodb.Configs{
	Table:            "my-table",
	Region:           "us-east-1",
	Endpoint:         "http://localhost:8000",
	PartitionKeyName: "pk",
})
```

### Health Check

The DynamoDB client provides health check functionality that verifies table accessibility:

```go
health, err := ctx.KVStore.HealthCheck(ctx)
if err != nil {
	// Handle health check failure
}
```

The health check returns a status ("UP" or "DOWN") along with table and region details.

### Data Storage Format

DynamoDB stores data in the following format:
- **Partition Key**: The key provided to Get/Set/Delete operations
- **Value Field**: A string field containing the JSON-serialized data

This design allows DynamoDB to work seamlessly with the GoFr KVStore interface while maintaining the flexibility to store complex data structures as JSON strings.