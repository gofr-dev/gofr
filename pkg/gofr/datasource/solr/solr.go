package solr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	Host string
	Port string
}

type Client struct {
	url string

	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// New initializes Solr driver with the provided configuration.
// The Connect method must be called to establish a connection to Solr.
// Usage:
// client := New(config)
// client.UseLogger(loggerInstance)
// client.UseMetrics(metricsInstance)
// client.Connect()
func New(conf Config) *Client {
	s := &Client{}
	s.url = "http://" + conf.Host + ":" + conf.Port + "/solr"

	return s
}

// UseLogger sets the logger for the Solr client which asserts the Logger interface.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Solr client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Solr client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// Connect establishes a connection to Solr and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	c.logger.Infof("connecting to Solr at %v", c.url)

	solrBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_solr_stats", "Response time of Solr operations in milliseconds.", solrBuckets...)

	return
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	return nil, nil
}

// Search searches documents in the given collections based on the parameters specified.
// This can be used for making any queries to SOLR
func (c *Client) Search(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/select"

	defer c.sendOperationStats(&QueryLog{Type: "Search", Url: url}, time.Now())

	return call(ctx, http.MethodGet, url, params, nil)
}

// Create makes documents in the specified collection. params can be used to send parameters like commit=true
func (c *Client) Create(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + collection + "/update"

	defer c.sendOperationStats(&QueryLog{Type: "Create", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, params, document)
}

// Update updates documents in the specified collection. params can be used to send parameters like commit=true
func (c *Client) Update(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + collection + "/update"

	defer c.sendOperationStats(&QueryLog{Type: "Update", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, params, document)
}

// Delete deletes documents in the specified collection. params can be used to send parameters like commit=true
func (c *Client) Delete(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + collection + "/update"

	defer c.sendOperationStats(&QueryLog{Type: "Delete", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, params, document)
}

// ListFields retrieves all the fields in the schema for the specified collection.
// params can be used to send query parameters like wt, fl, includeDynamic etc.
func (c *Client) ListFields(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + collection + "/schema/fields"

	defer c.sendOperationStats(&QueryLog{Type: "ListFields", Url: url}, time.Now())

	return call(ctx, http.MethodGet, url, params, nil)
}

// Retrieve retrieves the entire schema that includes all the fields,field types,dynamic rules and copy field rules.
// params can be used to specify the format of response
func (c *Client) Retrieve(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + collection + "/schema"

	defer c.sendOperationStats(&QueryLog{Type: "Retrieve", Url: url}, time.Now())

	return call(ctx, http.MethodGet, url, params, nil)
}

// AddField adds Field in the schema for the specified collection
func (c *Client) AddField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + collection + "/schema"

	defer c.sendOperationStats(&QueryLog{Type: "AddField", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, nil, document)
}

// UpdateField updates the field definitions in the schema for the specified collection
func (c *Client) UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + collection + "/schema"

	defer c.sendOperationStats(&QueryLog{Type: "UpdateField", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, nil, document)
}

// DeleteField deletes the field definitions in the schema for the specified collection
func (c *Client) DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + collection + "/schema"

	defer c.sendOperationStats(&QueryLog{Type: "DeleteField", Url: url}, time.Now())

	return call(ctx, http.MethodPost, url, nil, document)
}

// Response stores the response from SOLR
type Response struct {
	Code int
	Data any
}

// call forms the http request and makes a call to solr and populates the solr response
func call(ctx context.Context, method, url string, params map[string]any, body io.Reader) (any, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if method != http.MethodGet {
		req.Header.Add("content-type", "application/json")
	}

	q := req.URL.Query()

	for k, val := range params {
		switch v := val.(type) {
		case []string:
			for _, val := range v {
				q.Add(k, val)
			}
		default:
			q.Add(k, fmt.Sprint(val))
		}
	}

	req.URL.RawQuery = q.Encode()

	client := &http.Client{}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody any

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &respBody)
	if err != nil {
		return nil, err
	}

	return Response{resp.StatusCode, respBody}, nil
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_solr_stats", float64(duration),
		"type", ql.Type)
}
