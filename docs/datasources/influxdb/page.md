# InfluxDB
GoFr supports injecting InfluxDB using an interface that defines the necessary methods to interact with InfluxDB v2+.  
Any driver that implements this interface can be injected via the `app.AddInfluxDB()` method.

---

## Interface

```go
// InfluxDB defines the methods for interacting with an InfluxDB database.
type InfluxDB interface {
    CreateOrganization(ctx context.Context, orgName string) (string, error)
    DeleteOrganization(ctx context.Context, orgID string) error
    ListOrganization(ctx context.Context) (map[string]string, error)

    CreateBucket(ctx context.Context, orgID string, bucketName string, retentionPeriod time.Duration) (string, error)
    DeleteBucket(ctx context.Context, orgID, bucketID string) error
    ListBuckets(ctx context.Context, org string) (map[string]string, error)

    Ping(ctx context.Context) (bool, error)
    HealthCheck(ctx context.Context) (any, error)

    Query(ctx context.Context, org string, fluxQuery string) ([]map[string]any, error)
    WritePoints(ctx context.Context, bucket string, org string, points []container.InfluxPoint) error)
}
```

This structure supports all essential InfluxDB operations including organization/bucket management, health checks, and metrics ingestion.

Import the gofr's external driver for influxdb: 

```bash
go get gofr.dev/pkg/gofr/datasource/influxdb@latest
```

## Example
```go
package main

import (
"context"
"fmt"
"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/influxdb"
)

func main() {
	
    // Create a new GoFr application
    app := gofr.New() 
	
	// Initialize InfluxDB client
	client := influxdb.New(influxdb.Config{
		Url:      "http://localhost:8086",
		Username: "admin",
		Password: "admin1234",
		Token:    "<your-token>",
	})

	// Add InfluxDB to application context
	app.AddInfluxDB(client)

	// Sample route
	app.GET("/greet", func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	})

	// Ping InfluxDB
	ok, err := client.Ping(context.Background())
	if err != nil {
		app.Logger().Debug(err)
		return
	}
	app.Logger().Debug("InfluxDB connected: ", ok)

	// Create organization
	orgID, err := client.CreateOrganization(context.Background(), "demo-org")
	if err != nil {
		app.Logger().Debug(err)
		return
	}

	// List organizations
	orgs, _ := client.ListOrganization(context.Background())
	app.Logger().Debug("Organizations: ")
	for id, name := range orgs {
		app.Logger().Debug(id, name)
	}

	// Create bucket
	bucketID, err := client.CreateBucket(context.Background(), orgID, "demo-bucket")
	if err != nil {
		app.Logger().Debug(err)
		return
	}

	// List buckets for organization
	buckets, err := client.ListBuckets(context.Background(), "demo-org")
	if err != nil {
		app.Logger().Debug(err)
		return
	}
	app.Logger().Debug("Buckets:", buckets)

	// Delete bucket
	if err := client.DeleteBucket(context.Background(), bucketID); err != nil {
		app.Logger().Debug(err)
		return
	}
	app.Logger().Debug("Bucket deleted successfully")

	// Delete organization
	if err := client.DeleteOrganization(context.Background(), orgID); err != nil {
		app.Logger().Debug(err)
		return
	}
	app.Logger().Debug("Organization deleted successfully")
	// Start the server
	app.Run()
}
```
