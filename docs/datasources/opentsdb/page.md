# OpenTSDB


## Configuration
To connect to `OpenTSDB`, you need to provide the following environment variables:
- `HOSTS`: The hostname or IP address of your OpenTSDB server.
- `MAXCONTENTLENGTH`: Max length of the request body in bytes.
- `MAXPUTPOINTSNUM`: Max number of data points that can be sent in a single `PUT` request.
- `DETECTDELTANUM`: The number of data points that OpenTSDB looks at to spot unusual time gaps.

## Setup
GoFr supports injecting OpenTSDB to facilitate interaction with OpenTSDB's REST APIs.
Implementations adhering to the `OpenTSDB` interface can be registered with `app.AddOpenTSDB()`,
enabling applications to leverage OpenTSDB for time-series data management through `gofr.Context`.

```go
// OpenTSDB provides methods for GoFr applications to communicate with OpenTSDB
// through its REST APIs.
type OpenTSDB interface {
	// HealthChecker verifies if the OpenTSDB server is reachable.
	// Returns an error if the server is unreachable, otherwise nil.
	HealthChecker

	// PutDataPoints sends data to store metrics in OpenTSDB.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - data: A slice of DataPoint objects; must contain at least one entry.
	// - queryParam: Specifies the response format:
	//   - client.PutRespWithSummary: Requests a summary response.
	//   - client.PutRespWithDetails: Requests detailed response information.
	//   - Empty string (""): No additional response details.
	//
	// - res: A pointer to PutResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	PutDataPoints(ctx context.Context, data any, queryParam string, res any) error

	// QueryDataPoints retrieves data based on the specified parameters.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - param: An instance of QueryParam with query parameters for filtering data.
	// - res: A pointer to QueryResponse, where the server's response will be stored.
	QueryDataPoints(ctx context.Context, param any, res any) error

	// QueryLatestDataPoints fetches the latest data point(s).
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - param: An instance of QueryLastParam with query parameters for the latest data point.
	// - res: A pointer to QueryLastResponse, where the server's response will be stored.
	QueryLatestDataPoints(ctx context.Context, param any, res any) error

	// GetAggregators retrieves available aggregation functions.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - res: A pointer to AggregatorsResponse, where the server's response will be stored.
	GetAggregators(ctx context.Context, res any) error

	// QueryAnnotation retrieves a single annotation.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - queryAnnoParam: A map of parameters for the annotation query, such as client.AnQueryStartTime, client.AnQueryTSUid.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any, res any) error

	// PostAnnotation creates or updates an annotation.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be created or updated.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	PostAnnotation(ctx context.Context, annotation any, res any) error

	// PutAnnotation creates or replaces an annotation.
	// Fields not included in the request will be reset to default values.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be created or replaced.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	PutAnnotation(ctx context.Context, annotation any, res any) error

	// DeleteAnnotation removes an annotation.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be deleted.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	DeleteAnnotation(ctx context.Context, annotation any, res any) error
}
```

Import the gofr's external driver for OpenTSDB:

```go
go get gofr.dev/pkg/gofr/datasource/opentsdb
```

The following example demonstrates injecting an OpenTSDB instance into a GoFr application
and using it to perform a health check on the OpenTSDB server.
```go
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/opentsdb"
)

func main() {
	app := gofr.New()

	// Initialize OpenTSDB connection
	app.AddOpenTSDB(opentsdb.New(opentsdb.Config{
		Host:             app.Config.Get("HOST"),
		MaxContentLength: app.Config.Get("MAXCONTENTLENGTH"),
		MaxPutPointsNum:  app.Config.Get("MAXPUTPOINTSNUM"),
		DetectDeltaNum:   app.Config.Get("DETECTDELTANUM"),
	}))

	// Register routes
	app.GET("/health", opentsdbHealthCheck)
	app.POST("/write", writeDataPoints)
	app.GET("/query", queryDataPoints)
	// Run the app
	app.Run()
}

// Health check for OpenTSDB
func opentsdbHealthCheck(c *gofr.Context) (any, error) {
	res, err := c.OpenTSDB.HealthCheck(context.Background())
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Write Data Points to OpenTSDB
func writeDataPoints(c *gofr.Context) (any, error) {
	PutDataPointNum := 4
	name := []string{"cpu", "disk", "net", "mem"}
	cpuDatas := make([]opentsdb.DataPoint, 0)

	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for i := 0; i < PutDataPointNum; i++ {
		data := opentsdb.DataPoint{
			Metric:    name[i%len(name)],
			Timestamp: time.Now().Unix(),
			Value:     rand.Float64() * 100,
			Tags:      tags,
		}
		cpuDatas = append(cpuDatas, data)
	}

	resp := opentsdb.PutResponse{}

	err := c.OpenTSDB.PutDataPoints(context.Background(), cpuDatas, "details", &resp)
	if err != nil {
		return resp.Errors, err
	}

	return fmt.Sprintf("%v Data points written successfully", resp.Success), nil
}

// Query Data Points from OpenTSDB
func queryDataPoints(c *gofr.Context) (any, error) {
	st1 := time.Now().Unix() - 3600
	st2 := time.Now().Unix()

	queryParam := opentsdb.QueryParam{
		Start: st1,
		End:   st2,
	}

	name := []string{"cpu", "disk", "net", "mem"}
	subqueries := make([]opentsdb.SubQuery, 0)
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for _, metric := range name {
		subQuery := opentsdb.SubQuery{
			Aggregator: "sum",
			Metric:     metric,
			Tags:       tags,
		}
		subqueries = append(subqueries, subQuery)
	}

	queryParam.Queries = subqueries

	queryResp := &opentsdb.QueryResponse{}

	err := c.OpenTSDB.QueryDataPoints(c, &queryParam, queryResp)
	if err != nil {
		return nil, err
	}
	return queryResp.QueryRespCnts, nil
}
```
