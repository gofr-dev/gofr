// Package opentsdb provides a client implementation for interacting with OpenTSDB
// via its REST API.
package opentsdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Predefined static errors.
var (
	errInvalidResponseType = errors.New("invalid response type")
	errInvalidQueryParam   = errors.New("invalid query parameters")
	errInvalidParam        = errors.New("invalid parameter type")
)

const (
	statusFailed      = "FAIL"
	statusSuccess     = "SUCCESS"
	defaultDialTime   = 5 * time.Second  // Default time for establishing TCP connections.
	connectionTimeout = 30 * time.Second // Timeout for keeping connections alive.

	// API paths for OpenTSDB endpoints.
	putPath        = "/api/put"
	aggregatorPath = "/api/aggregators"
	versionPath    = "/api/version"
	annotationPath = "/api/annotation"
	queryPath      = "/api/query"
	queryLastPath  = "/api/query/last"

	putRespWithSummary = "summary" // Summary response for PUT operations.
	putRespWithDetails = "details" // Detailed response for PUT operations.

	// The three keys in the rateOption parameter of the QueryParam.
	queryRateOptionCounter    = "counter"    // The corresponding value type is bool
	queryRateOptionCounterMax = "counterMax" // The corresponding value type is int,int64
	queryRateOptionResetValue = "resetValue" // The corresponding value type is int,int64

	anQueryStartTime = "start_time"
	anQueryTSUid     = "tsuid"

	// The below three constants are used in /put.
	defaultMaxPutPointsNum = 75
	defaultDetectDeltaNum  = 3
	// Unit is bytes, and assumes that config items of 'tsd.http.request.enable_chunked = true'
	// and 'tsd.http.request.max_chunk = 40960' are all in the opentsdb.conf.
	defaultMaxContentLength = 40960

	opentsdbOperationDurationName = "app_opentsdb_operation_duration"
	opentsdbOperationTotalName    = "app_opentsdb_operation_total"
)

//nolint:gochecknoglobals // this variable is being set again with a mockserver response for testing HealthCheck endpoint.
var dialTimeout = net.DialTimeout

// Client is the implementation of the OpenTSDBClient interface,
// which includes context-aware functionality.
type Client struct {
	endpoint string
	client   httpClient
	config   Config
	logger   Logger
	metrics  Metrics
	tracer   trace.Tracer
}

type Config struct {

	// The host of the target opentsdb, is a required non-empty string which is
	// in the format of ip:port without http:// prefix or a domain.
	Host string

	// A pointer of http.Transport is used by the opentsdb client.
	// This value is optional, and if it is not set, client.defaultTransport, which
	// enables tcp keepalive mode, will be used in the opentsdb client.
	Transport *http.Transport

	// The maximal number of datapoints which will be inserted into the opentsdb
	// via one calling of /api/put method.
	// This value is optional, and if it is not set, client.defaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxPutPointsNum int

	// The detect delta number of datapoints which will be used in client.Put()
	// to split a large group of datapoints into small batches.
	// This value is optional, and if it is not set, client.defaultDetectDeltaNum
	// will be used in the opentsdb client.
	DetectDeltaNum int

	// The maximal body content length per /api/put method to insert datapoints
	// into opentsdb.
	// This value is optional, and if it is not set, client.defaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxContentLength int
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// New initializes a new instance of Opentsdb with provided configuration.
func New(config Config) *Client {
	return &Client{config: config}
}

func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// registerMetrics initializes OpenTSDB metrics once when a metrics provider is injected.
func (c *Client) registerMetrics() {
	if c.metrics == nil {
		return
	}

	durationBuckets := []float64{
		1,    // 1 ms
		5,    // 5 ms
		10,   // 10 ms
		50,   // 50 ms
		100,  // 100 ms
		250,  // 250 ms
		500,  // 500 ms
		1000, // 1 s
		2000, // 2 s
		5000, // 5 s
	}

	c.metrics.NewHistogram(
		opentsdbOperationDurationName,
		"Duration of OpenTSDB operations in milliseconds.",
		durationBuckets...,
	)

	c.metrics.NewCounter(
		opentsdbOperationTotalName,
		"Total OpenTSDB operations.",
	)
}

// Connect initializes an HTTP client for OpenTSDB using the provided configuration.
// If the configuration is invalid or the endpoint is unreachable, an error is logged.
func (c *Client) Connect() {
	c.registerMetrics()

	span := c.addTrace(context.Background(), "Connect")

	if span != nil {
		span.SetAttributes(attribute.Int64(fmt.Sprintf("opentsdb.%v", "Connect"), 0))
		span.End()
	}

	c.logger.Debugf("connecting to OpenTSDB at host %s", c.config.Host)

	// Set default values for optional configuration fields.
	c.initializeClient()

	// Initialize the OpenTSDB client with the given configuration.
	c.endpoint = fmt.Sprintf("http://%s", c.config.Host)

	res := VersionResponse{}

	err := c.version(context.Background(), &res)
	if err != nil {
		c.logger.Errorf("error while connecting to OpenTSDB: %v", err)
		return
	}

	c.logger.Logf("connected to OpenTSDB at %s", c.endpoint)
}

func (c *Client) PutDataPoints(ctx context.Context, datas any, queryParam string, resp any) error {
	span := c.addTrace(ctx, "PutDataPoints")

	status := statusFailed

	message := "Put request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "PutDataPoints", &status, &message, span)

	putResp, ok := resp.(*PutResponse)
	if !ok {
		return fmt.Errorf("%w: Must be *PutResponse", errInvalidResponseType)
	}

	datapoints, ok := datas.([]DataPoint)
	if !ok {
		return fmt.Errorf("%w: Must be []DataPoint", errInvalidResponseType)
	}

	err := validateDataPoint(datapoints)
	if err != nil {
		message = err.Error()
		return err
	}

	if !isValidPutParam(queryParam) {
		message = "the given query param is invalid."
		return errInvalidQueryParam
	}

	putEndpoint := fmt.Sprintf("%s%s", c.endpoint, putPath)
	if !isEmptyPutParam(queryParam) {
		putEndpoint = fmt.Sprintf("%s?%s", putEndpoint, queryParam)
	}

	tempResp, err := c.getResponse(ctx, putEndpoint, datapoints, &message)
	if err != nil {
		return err
	}

	if len(tempResp.Errors) > 0 {
		return parsePutErrorMsg(tempResp)
	}

	status = statusSuccess
	message = fmt.Sprintf("put request to url %q processed successfully", putEndpoint)
	*putResp = *tempResp

	return nil
}

func (c *Client) QueryDataPoints(ctx context.Context, parameters, resp any) error {
	span := c.addTrace(ctx, "QueryDataPoints")

	status := statusFailed

	message := "QueryDatapoints request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "Query", &status, &message, span)

	param, ok := parameters.(*QueryParam)
	if !ok {
		return fmt.Errorf("%w: Must be *QueryParam", errInvalidQueryParam)
	}

	queryResp, ok := resp.(*QueryResponse)
	if !ok {
		return fmt.Errorf("%w: Must be *QueryResponse", errInvalidResponseType)
	}

	if !isValidQueryParam(param) {
		message = "invalid query parameters"
		return errInvalidQueryParam
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.endpoint, queryPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprintf("getQueryBodyContents error: %s", err)
		return err
	}

	if err = c.sendRequest(ctx, http.MethodPost, queryEndpoint, reqBodyCnt, queryResp); err != nil {
		message = fmt.Sprintf("error processing Query request at url %q: %s ", queryEndpoint, err)
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("query request at url %q processed successfully", queryEndpoint)

	return nil
}

func (c *Client) QueryLatestDataPoints(ctx context.Context, parameters, resp any) error {
	span := c.addTrace(ctx, "QueryLastDataPoints")

	status := statusFailed

	message := "QueryLatestDataPoints request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "QueryLastDataPoints", &status, &message, span)

	param, ok := parameters.(*QueryLastParam)
	if !ok {
		return fmt.Errorf("%w: Must be a *QueryLastParam type", errInvalidParam)
	}

	queryResp, ok := resp.(*QueryLastResponse)
	if !ok {
		return fmt.Errorf("%w: Must be a *QueryLastResponse type", errInvalidResponseType)
	}

	if !isValidQueryLastParam(param) {
		message = "invalid query last param"
		return errInvalidQueryParam
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.endpoint, queryLastPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprint("error retrieving body contents: ", err)
		return err
	}

	if err = c.sendRequest(ctx, http.MethodPost, queryEndpoint, reqBodyCnt, queryResp); err != nil {
		message = fmt.Sprintf("error processing LatestQuery request at url %q: %s ", queryEndpoint, err)
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("querylast request to url %q processed successfully", queryEndpoint)

	c.logger.Logf("querylast request processed successfully")

	return nil
}

func (c *Client) QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any, resp any) error {
	span := c.addTrace(ctx, "QueryAnnotation")

	status := statusFailed

	message := "QueryAnnotation request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "QueryAnnotation", &status, &message, span)

	annResp, ok := resp.(*AnnotationResponse)
	if !ok {
		return fmt.Errorf("%w: Must be *AnnotationResponse", errInvalidResponseType)
	}

	if len(queryAnnoParam) == 0 {
		message = "annotation query parameter is empty"
		return fmt.Errorf("%w: %s", errInvalidQueryParam, message)
	}

	buffer := bytes.NewBuffer(nil)

	queryURL := url.Values{}

	for k, v := range queryAnnoParam {
		value, ok := v.(string)
		if ok {
			queryURL.Add(k, value)
		}
	}

	buffer.WriteString(queryURL.Encode())

	annoEndpoint := fmt.Sprintf("%s%s?%s", c.endpoint, annotationPath, buffer.String())

	if err := c.sendRequest(ctx, http.MethodGet, annoEndpoint, "", annResp); err != nil {
		message = fmt.Sprintf("error processing AnnotationQuery request: %s", err.Error())
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("Annotation query sent to url: %s", annoEndpoint)

	c.logger.Log("Annotation query processed successfully")

	return nil
}

func (c *Client) PostAnnotation(ctx context.Context, annotation, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodPost, "PostAnnotation")
}

func (c *Client) PutAnnotation(ctx context.Context, annotation, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodPut, "PutAnnotation")
}

func (c *Client) DeleteAnnotation(ctx context.Context, annotation, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodDelete, "DeleteAnnotation")
}

func (c *Client) GetAggregators(ctx context.Context, resp any) error {
	span := c.addTrace(ctx, "GetAggregators")

	status := statusFailed

	message := "GetAggregators request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "GetAggregators", &status, &message, span)

	aggreResp, ok := resp.(*AggregatorsResponse)
	if !ok {
		return fmt.Errorf("%w: Must be a *AggregatorsResponse", errInvalidResponseType)
	}

	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.endpoint, aggregatorPath)

	if err := c.sendRequest(ctx, http.MethodGet, aggregatorsEndpoint, "", aggreResp); err != nil {
		message = fmt.Sprintf("error retrieving aggregators from url: %s", aggregatorsEndpoint)
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("aggregators retrieved from url: %s", aggregatorsEndpoint)

	c.logger.Log("aggregators fetched successfully")

	return nil
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	span := c.addTrace(ctx, "HealthCheck")

	status := statusFailed

	message := "HealthCheck request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "HealthCheck", &status, &message, span)

	h := Health{
		Details: make(map[string]any),
	}

	conn, err := dialTimeout("tcp", c.config.Host, defaultDialTime)
	if err != nil {
		h.Status = "DOWN"
		message = fmt.Sprintf("OpenTSDB is unreachable: %v", err)

		return nil, err
	}

	if conn != nil {
		defer conn.Close()
	}

	h.Details["host"] = c.endpoint

	ver := &VersionResponse{}

	err = c.version(ctx, ver)
	if err != nil {
		message = err.Error()
		return nil, err
	}

	h.Details["version"] = ver.VersionInfo["version"]

	status = statusSuccess
	h.Status = "UP"
	message = "connection to OpenTSDB is alive"

	return &h, nil
}
