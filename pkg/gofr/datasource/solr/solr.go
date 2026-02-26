package solr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	Host string
	Port string
}

type Client struct {
	url     string
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
	client  *http.Client
}

// New initializes Solr driver with the provided configuration.
// The Connect method must be called to establish a connection to Solr.
// Usage:
// client := New(config)
// client.UseLogger(loggerInstance)
// client.UseMetrics(metricsInstance)
// client.Connect().
func New(conf Config) *Client {
	s := &Client{}
	s.url = "http://" + conf.Host + ":" + conf.Port + "/solr"
	s.client = &http.Client{}

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
	c.logger.Debugf("connecting to Solr at %v", c.url)

	solrBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_solr_stats", "Response time of Solr operations in milliseconds.", solrBuckets...)

	_, err := c.HealthCheck(context.Background())
	if err != nil {
		c.logger.Errorf("error while connecting to Solr: %v", err)
		return
	}

	c.logger.Infof("connected to Solr at %v", c.url)
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	url := c.url + "/admin/info/system?wt=json"

	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodGet, url, nil, nil)

	defer c.sendOperationStats(ctx, &QueryLog{Type: "HealthCheck", URL: url}, startTime, "healthcheck", span)

	return resp, err
}

// Search searches documents in the given collections based on the parameters specified.
// This can be used for making any queries to Solr.
func (c *Client) Search(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/select"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodGet, url, params, nil)

	c.sendOperationStats(ctx, &QueryLog{Type: "Search", URL: url}, startTime, "search", span)

	return resp, err
}

// Create makes documents in the specified collection. params can be used to send parameters like commit=true.
func (c *Client) Create(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/update"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, params, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "Create", URL: url}, startTime, "create", span)

	return resp, err
}

// Update updates documents in the specified collection. params can be used to send parameters like commit=true.
func (c *Client) Update(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/update"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, params, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "Update", URL: url}, startTime, "update", span)

	return resp, err
}

// Delete deletes documents in the specified collection. params can be used to send parameters like commit=true.
func (c *Client) Delete(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/update"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, params, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "Delete", URL: url}, startTime, "delete", span)

	return resp, err
}

// ListFields retrieves all the fields in the schema for the specified collection.
// params can be used to send query parameters like wt, fl, includeDynamic etc.
func (c *Client) ListFields(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/schema/fields"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodGet, url, params, nil)

	c.sendOperationStats(ctx, &QueryLog{Type: "ListFields", URL: url}, startTime, "list-fields", span)

	return resp, err
}

// Retrieve retrieves the entire schema that includes all the fields,field types,dynamic rules and copy field rules.
// params can be used to specify the format of response.
func (c *Client) Retrieve(ctx context.Context, collection string, params map[string]any) (any, error) {
	url := c.url + "/" + collection + "/schema"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodGet, url, params, nil)

	c.sendOperationStats(ctx, &QueryLog{Type: "Retrieve", URL: url}, startTime, "retrieve", span)

	return resp, err
}

// AddField adds Field in the schema for the specified collection.
func (c *Client) AddField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + "/" + collection + "/schema"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, nil, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "AddField", URL: url}, startTime, "add-field", span)

	return resp, err
}

// UpdateField updates the field definitions in the schema for the specified collection.
func (c *Client) UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + "/" + collection + "/schema"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, nil, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "UpdateField", URL: url}, startTime, "update-field", span)

	return resp, err
}

// DeleteField deletes the field definitions in the schema for the specified collection.
func (c *Client) DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (any, error) {
	url := c.url + "/" + collection + "/schema"
	startTime := time.Now()

	resp, span, err := c.call(ctx, http.MethodPost, url, nil, document)

	c.sendOperationStats(ctx, &QueryLog{Type: "DeleteField", URL: url}, startTime, "delete-field", span)

	return resp, err
}

// Response stores the response from Solr.
type Response struct {
	Code int
	Data any
}

// call forms the http request and makes a call to solr and populates the solr response.
func (c *Client) call(ctx context.Context, method, url string, params map[string]any, body io.Reader) (any, trace.Span, error) {
	var span trace.Span

	if c.tracer != nil {
		ctx, span = c.tracer.Start(ctx, fmt.Sprintf("Solr %s", method),
			trace.WithAttributes(
				attribute.String("solr.url", url),
			),
		)
	}

	ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))

	req, err := c.createRequest(ctx, method, url, params, body)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var respBody any

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(b, &respBody)
	if err != nil {
		return nil, nil, err
	}

	if span != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
		)
	}

	return Response{resp.StatusCode, respBody}, span, nil
}

func (*Client) createRequest(ctx context.Context, method, url string, params map[string]any, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	if method != http.MethodGet {
		req.Header.Add("Content-Type", "application/json")
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

	return req, nil
}

func (c *Client) sendOperationStats(ctx context.Context, ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(ctx, "app_solr_stats", float64(duration),
		"type", ql.Type)

	if span != nil {
		defer span.End()

		span.SetAttributes(
			attribute.String("solr.type", ql.Type),
			attribute.Int64(fmt.Sprintf("solr.%v.duration", method), duration))
	}
}
