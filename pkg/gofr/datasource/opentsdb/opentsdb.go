// Package opentsdb provides a client implementation for interacting with OpenTSDB
// via its REST API.
package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	StatusFailed      = "FAIL"
	StatusSuccess     = "SUCCESS"
	DefaultDialTime   = 5 * time.Second  // Default time for establishing TCP connections.
	ConnectionTimeout = 30 * time.Second // Timeout for keeping connections alive.

	// API paths for OpenTSDB endpoints.
	PutPath        = "/api/put"
	AggregatorPath = "/api/aggregators"
	VersionPath    = "/api/version"
	AnnotationPath = "/api/annotation"
	QueryPath      = "/api/query"
	QueryLastPath  = "/api/query/last"

	PutRespWithSummary = "summary" // Summary response for PUT operations.
	PutRespWithDetails = "details" // Detailed response for PUT operations.

	// The three keys in the rateOption parameter of the QueryParam.
	QueryRateOptionCounter    = "counter"    // The corresponding value type is bool
	QueryRateOptionCounterMax = "counterMax" // The corresponding value type is int,int64
	QueryRateOptionResetValue = "resetValue" // The corresponding value type is int,int64

	AnQueryStartTime = "start_time"
	AnQueryTSUid     = "tsuid"

	// The below three constants are used in /put.
	DefaultMaxPutPointsNum = 75
	DefaultDetectDeltaNum  = 3
	// Unit is bytes, and assumes that config items of 'tsd.http.request.enable_chunked = true'
	// and 'tsd.http.request.max_chunk = 40960' are all in the opentsdb.conf.
	DefaultMaxContentLength = 40960
)

var dialTimeout = net.DialTimeout

// Client is the implementation of the OpenTSDBClient interface,
// which includes context-aware functionality.
type Client struct {
	endpoint string
	client   HTTPClient
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
	// This value is optional, and if it is not set, client.DefaultTransport, which
	// enables tcp keepalive mode, will be used in the opentsdb client.
	Transport *http.Transport

	// The maximal number of datapoints which will be inserted into the opentsdb
	// via one calling of /api/put method.
	// This value is optional, and if it is not set, client.DefaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxPutPointsNum int

	// The detect delta number of datapoints which will be used in client.Put()
	// to split a large group of datapoints into small batches.
	// This value is optional, and if it is not set, client.DefaultDetectDeltaNum
	// will be used in the opentsdb client.
	DetectDeltaNum int

	// The maximal body content length per /api/put method to insert datapoints
	// into opentsdb.
	// This value is optional, and if it is not set, client.DefaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxContentLength int
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// New initializes a new instance of Opentsdb with provided configuration.
func New(config *Config) *Client {
	return &Client{config: *config}
}

func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// DefaultTransport defines the default HTTP transport settings,
// including connection timeouts and idle connections.
var DefaultTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	DialContext: (&net.Dialer{
		Timeout:   DefaultDialTime,
		KeepAlive: ConnectionTimeout,
	}).DialContext,
}

// Connect initializes an HTTP client for OpenTSDB using the provided configuration.
// If the configuration is invalid or the endpoint is unreachable, an error is logged.
func (c *Client) Connect() {
	span := c.addTrace(context.Background(), "Connect")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Connect", &status, &message, span)

	c.config.Host = strings.TrimSpace(c.config.Host)
	if c.config.Host == "" {
		c.logger.Errorf("the OpentsdbEndpoint in the given configuration cannot be empty.")
	}

	// Use custom transport settings if provided, otherwise, use the default transport.
	transport := c.config.Transport
	if transport == nil {
		transport = DefaultTransport
	}

	c.client = &http.Client{
		Transport: transport,
	}

	// Set default values for optional configuration fields.
	if c.config.MaxPutPointsNum <= 0 {
		c.config.MaxPutPointsNum = DefaultMaxPutPointsNum
	}

	if c.config.DetectDeltaNum <= 0 {
		c.config.DetectDeltaNum = DefaultDetectDeltaNum
	}

	if c.config.MaxContentLength <= 0 {
		c.config.MaxContentLength = DefaultMaxContentLength
	}

	// Initialize the OpenTSDB client with the given configuration.
	c.endpoint = fmt.Sprintf("http://%s", c.config.Host)

	c.logger.Logf("Connection Successful")

	status = StatusSuccess
	message = fmt.Sprintf("connected to %s", c.endpoint)
}

func (c *Client) PutDataPoints(ctx context.Context, datas any, queryParam string, resp any) error {
	span := c.addTrace(ctx, "Put")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Put", &status, &message, span)

	putResp, ok := resp.(*PutResponse)
	if !ok {
		return errors.New("invalid response type. Must be *PutResponse")
	}

	datapoints, ok := datas.([]DataPoint)
	if !ok {
		return errors.New("invalid response type. Must be []DataPoint")
	}

	err := validateDataPoint(datapoints)
	if err != nil {
		message = fmt.Sprintf("invalid data: %s", err)
		return err
	}

	if !isValidPutParam(queryParam) {
		message = "The given query param is invalid."
		return errors.New(message)
	}

	var putEndpoint string
	if !isEmptyPutParam(queryParam) {
		putEndpoint = fmt.Sprintf("%s%s?%s", c.endpoint, PutPath, queryParam)
	} else {
		putEndpoint = fmt.Sprintf("%s%s", c.endpoint, PutPath)
	}

	tempResp, err := c.getResponse(ctx, putEndpoint, datapoints, &message)
	if err != nil {
		return err
	}

	if len(tempResp.Errors) == 0 {
		status = StatusSuccess
		message = fmt.Sprintf("Put request to url %q processed successfully", putEndpoint)
		putResp.Success = tempResp.Success
		putResp.Failed = tempResp.Failed
		putResp.Errors = tempResp.Errors

		return nil
	}

	return parsePutErrorMsg(tempResp)
}

func (c *Client) QueryDataPoints(ctx context.Context, parameters any, resp any) error {
	span := c.addTrace(ctx, "Query")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Query", &status, &message, span)

	param, ok := parameters.(*QueryParam)
	if !ok {
		return errors.New("invalid parameter type")
	}

	queryResp, ok := resp.(*QueryResponse)
	if !ok {
		return errors.New("invalid response type")
	}

	if param.tracer == nil {
		param.tracer = c.tracer
	}

	if param.logger == nil {
		param.logger = c.logger
	}

	if !isValidQueryParam(param) {
		message = "invalid query parameters"
		return errors.New(message)
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.endpoint, QueryPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprintf("getQueryBodyContents error: %s", err)
		return err
	}

	queryResp.logger = c.logger
	queryResp.tracer = c.tracer
	queryResp.ctx = ctx

	if err = c.sendRequest(ctx, http.MethodPost, queryEndpoint, reqBodyCnt, queryResp); err != nil {
		message = fmt.Sprintf("error while processing request at url %q: %s ", queryEndpoint, err)
		return err
	}

	status = StatusSuccess
	message = fmt.Sprintf("query request at url %q processed successfully", queryEndpoint)

	return nil
}

func (c *Client) QueryLatestDataPoints(ctx context.Context, parameters any, resp any) error {
	param, ok := parameters.(*QueryLastParam)
	if !ok {
		return errors.New("invalid parameter type. Must be a *QueryLastParam type")
	}

	queryResp, ok := resp.(*QueryLastResponse)
	if !ok {
		return errors.New("invalid response type. Must be a *QueryLastResponse type")
	}

	span := c.addTrace(ctx, "QueryLast")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryLast", &status, &message, span)

	if !isValidQueryLastParam(param) {
		message = "invalid query last param"
		return errors.New(message)
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.endpoint, QueryLastPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprint("error retrieving body contents: ", err)
		return err
	}

	queryResp.logger = c.logger
	queryResp.tracer = c.tracer
	queryResp.ctx = ctx

	if err = c.sendRequest(ctx, http.MethodPost, queryEndpoint, reqBodyCnt, queryResp); err != nil {
		message = fmt.Sprintf("error sending request at url %s : %s ", queryEndpoint, err)
		return err
	}

	status = StatusSuccess
	message = fmt.Sprintf("querylast request to url %q processed successfully", queryEndpoint)

	c.logger.Logf("querylast request processed successfully")

	return nil
}

func (c *Client) QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any, resp any) error {
	span := c.addTrace(ctx, "QueryAnnotation")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryAnnotation", &status, &message, span)

	annResp, ok := resp.(*AnnotationResponse)
	if !ok {
		return errors.New("invalid response type. Must be *AnnotationResponse")
	}

	if len(queryAnnoParam) == 0 {
		message = "annotation query parameter is empty"
		return errors.New(message)
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

	annoEndpoint := fmt.Sprintf("%s%s?%s", c.endpoint, AnnotationPath, buffer.String())
	annResp.logger = c.logger
	annResp.tracer = c.tracer
	annResp.ctx = ctx

	if err := c.sendRequest(ctx, http.MethodGet, annoEndpoint, "", annResp); err != nil {
		message = fmt.Sprintf("error while processing annotation query: %s", err.Error())
		return err
	}

	status = StatusSuccess
	message = fmt.Sprintf("Annotation query sent to url: %s", annoEndpoint)

	c.logger.Log("Annotation query processed successfully")

	return nil
}

func (c *Client) PostAnnotation(ctx context.Context, annotation any, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodPost, "PostAnnotation")
}

func (c *Client) PutAnnotation(ctx context.Context, annotation any, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodPut, "PutAnnotation")
}

func (c *Client) DeleteAnnotation(ctx context.Context, annotation any, resp any) error {
	return c.operateAnnotation(ctx, annotation, resp, http.MethodDelete, "DeleteAnnotation")
}

func (c *Client) GetAggregators(ctx context.Context, resp any) error {
	span := c.addTrace(ctx, "Aggregators")
	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Aggregators", &status, &message, span)

	aggreResp, ok := resp.(*AggregatorsResponse)
	if !ok {
		return errors.New("invalid response type. Must be a *AggregatorsResponse")
	}

	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.endpoint, AggregatorPath)

	aggreResp.logger = c.logger
	aggreResp.tracer = c.tracer
	aggreResp.ctx = ctx

	if err := c.sendRequest(ctx, http.MethodGet, aggregatorsEndpoint, "", aggreResp); err != nil {
		message = fmt.Sprintf("error retrieving aggregators from url: %s", aggregatorsEndpoint)
		return err
	}

	status = StatusSuccess
	message = fmt.Sprintf("aggregators retrieved from url: %s", aggregatorsEndpoint)

	c.logger.Log("aggregators fetched successfully")

	return nil
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	span := c.addTrace(ctx, "HealthCheck")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "HealthCheck", &status, &message, span)

	h := Health{
		Details: make(map[string]any),
	}

	conn, err := dialTimeout("tcp", c.config.Host, DefaultDialTime)
	if err != nil {
		h.Status = "DOWN"
		message = fmt.Sprintf("OpenTSDB is unreachable: %v", err)

		return nil, errors.New(message)
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

	status = StatusSuccess
	h.Status = "UP"
	message = "connection to OpenTSDB is alive"

	return &h, nil
}

func (c *Client) operateAnnotation(ctx context.Context, queryAnnotation any, resp any, method, operation string) error {
	span := c.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), operation, &status, &message, span)

	annotation, ok := queryAnnotation.(*Annotation)
	if !ok {
		return errors.New("invalid annotation type. Must be *Annotation")
	}

	annResp, ok := resp.(*AnnotationResponse)
	if !ok {
		return errors.New("invalid response type. Must be *AnnotationResponse")
	}

	if !c.isValidOperateMethod(ctx, method) {
		message = fmt.Sprintf("invalid annotation operation method: %s", method)
		return errors.New(message)
	}

	annoEndpoint := fmt.Sprintf("%s%s", c.endpoint, AnnotationPath)

	resultBytes, err := json.Marshal(annotation)
	if err != nil {
		message = fmt.Sprintf("marshal annotation response error: %s", err)
		return errors.New(message)
	}

	annResp.logger = c.logger
	annResp.tracer = c.tracer
	annResp.ctx = ctx

	if err = c.sendRequest(ctx, method, annoEndpoint, string(resultBytes), annResp); err != nil {
		message = fmt.Sprintf("%s: error while processing %s annotation request to url %q: %s", operation, method, annoEndpoint, err.Error())
		return err
	}

	status = StatusSuccess
	message = fmt.Sprintf("%s: %s annotation request to url %q processed successfully", operation, method, annoEndpoint)

	c.logger.Log("%s request successful", operation)

	return nil
}
