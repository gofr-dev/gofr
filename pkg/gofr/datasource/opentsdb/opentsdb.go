// Package opentsdb provides a client implementation for interacting with OpenTSDB
// via its REST API. The core client functionality is defined in opentsdb.go,
// while specific API methods are handled in separate files (e.g., put.go, query.go).
package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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
	PutPath            = "/api/put"
	PutRespWithSummary = "summary" // Summary response for PUT operations.
	PutRespWithDetails = "details" // Detailed response for PUT operations.
	QueryPath          = "/api/query"
	QueryLastPath      = "/api/query/last"

	// The three keys in the rateOption parameter of the QueryParam.
	QueryRateOptionCounter    = "counter"    // The corresponding value type is bool
	QueryRateOptionCounterMax = "counterMax" // The corresponding value type is int,int64
	QueryRateOptionResetValue = "resetValue" // The corresponding value type is int,int64

	AggregatorPath = "/api/aggregators"
	SuggestPath    = "/api/suggest"
	// Only the one of the three query type can be used in SuggestParam, UIDMetaData.
	TypeMetrics = "metrics"
	TypeTagk    = "tagk"
	TypeTagv    = "tagv"

	VersionPath        = "/api/version"
	DropcachesPath     = "/api/dropcaches"
	AnnotationPath     = "/api/annotation"
	AnQueryStartTime   = "start_time"
	AnQueryTSUid       = "tsuid"
	BulkAnnotationPath = "/api/annotation/bulk"
	UIDMetaDataPath    = "/api/uid/uidmeta"
	UIDAssignPath      = "/api/uid/assign"
	TSMetaDataPath     = "/api/uid/tsmeta"

	// The above three constants are used in /put.
	DefaultMaxPutPointsNum = 75
	DefaultDetectDeltaNum  = 3
	// Unit is bytes, and assumes that config items of 'tsd.http.request.enable_chunked = true'
	// and 'tsd.http.request.max_chunk = 40960' are all in the opentsdb.conf.
	DefaultMaxContentLength = 40960
)

var dialTimeout = net.DialTimeout

// Client is the implementation of the OpentsDBClient interface,
// which includes context-aware functionality.
type Client struct {
	endpoint string
	client   HTTPClient
	ctx      context.Context
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
	if c.ctx == nil {
		c.ctx = context.Background()
	}

	span := c.addTrace(c.ctx, "Connect")

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

// WithContext creates a new OpenTSDB client that operates with the provided context.
func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{
		endpoint: c.endpoint,
		client:   c.client,
		ctx:      ctx,
		config:   c.config,
	}
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(_ context.Context) (any, error) {
	span := c.addTrace(c.ctx, "HealthCheck")

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

	ver, err := c.version()
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

// sendRequest dispatches an HTTP request to the OpenTSDB server, using the provided
// method, URL, and body content. It returns the parsed response or an error, if any.
func (c *Client) sendRequest(method, url, reqBodyCnt string, parsedResp Response) error {
	span := c.addTrace(c.ctx, "sendRequest")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "sendRequest", &status, &message, span)

	// Create the HTTP request, attaching the context if available.
	req, err := http.NewRequest(method, url, strings.NewReader(reqBodyCnt))
	if c.ctx != nil {
		req = req.WithContext(c.ctx)
	}

	if err != nil {
		errRequestCreation := fmt.Errorf("failed to create request for %s %s: %w", method, url, err)

		message = fmt.Sprint(errRequestCreation)

		return errRequestCreation
	}

	// Set the request headers.
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	// Send the request and handle the response.
	resp, err := c.client.Do(req)
	if err != nil {
		errSendingRequest := fmt.Errorf("failed to send request for %s %s: %w", method, url, err)

		message = fmt.Sprint(errSendingRequest)

		return errSendingRequest
	}

	defer resp.Body.Close()

	// Read and parse the response.
	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		errReading := fmt.Errorf("failed to read response body for %s %s: %w", method, url, err)

		message = fmt.Sprint(errReading)

		return errReading
	}

	parsedResp.SetStatus(resp.StatusCode)

	parser := parsedResp.GetCustomParser()
	if parser == nil {
		// Use the default JSON unmarshaller if no custom parser is provided.
		if err := json.Unmarshal(jsonBytes, parsedResp); err != nil {
			errUnmarshaling := fmt.Errorf("failed to unmarshal response body for %s %s: %w", method, url, err)

			message = fmt.Sprint(errUnmarshaling)

			return errUnmarshaling
		}
	} else {
		// Use the custom parser if available.
		if err := parser(jsonBytes); err != nil {
			message = fmt.Sprintf("failed to parse response body through custom parser %s %s: %v", method, url, err)
			return err
		}
	}

	status = StatusSuccess
	message = fmt.Sprintf("%s request sent at : %s", method, url)

	return nil
}

// isValidOperateMethod checks if the provided HTTP method is valid for
// operations such as POST, PUT, or DELETE.
func (c *Client) isValidOperateMethod(method string) bool {
	span := c.addTrace(c.ctx, "isValidOperateMethod")

	status := StatusSuccess

	var message string

	defer sendOperationStats(c.logger, time.Now(), "isValidOperateMethod", &status, &message, span)

	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return false
	}

	validMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, validMethod := range validMethods {
		if method == validMethod {
			return true
		}
	}

	return false
}
