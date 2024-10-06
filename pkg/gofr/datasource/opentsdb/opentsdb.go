// Package opentsdb provides a client implementation for interacting with OpenTSDB
// via its REST API. The core client functionality is defined in opentsdb.go,
// while specific API methods are handled in separate files (e.g., put.go, query.go).
package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultDialTime   = 5 * time.Second  // Default time for establishing TCP connections.
	ConnectionTimeout = 30 * time.Second // Timeout for keeping connections alive.
	GetMethod         = "GET"            // HTTP GET method.
	PostMethod        = "POST"           // HTTP POST method.
	PutMethod         = "PUT"            // HTTP PUT method.
	DeleteMethod      = "DELETE"         // HTTP DELETE method.

	// API paths for OpenTSDB endpoints.
	PutPath            = "/api/put"
	PutRespWithSummary = "summary" // Summary response for PUT operations.
	PutRespWithDetails = "details" // Detailed response for PUT operations.
	QueryPath          = "/api/query"
	QueryLastPath      = "/api/query/last"

	// The three keys in the rateOption parameter of the QueryParam
	QueryRateOptionCounter    = "counter"    // The corresponding value type is bool
	QueryRateOptionCounterMax = "counterMax" // The corresponding value type is int,int64
	QueryRateOptionResetValue = "resetValue" // The corresponding value type is int,int64

	AggregatorPath  = "/api/aggregators"
	ConfigPath      = "/api/config"
	SerializersPath = "/api/serializers"
	StatsPath       = "/api/stats"
	SuggestPath     = "/api/suggest"
	// Only the one of the three query type can be used in SuggestParam, UIDMetaData:
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

	// The above three constants are used in /put
	DefaultMaxPutPointsNum = 75
	DefaultDetectDeltaNum  = 3
	// Unit is bytes, and assumes that config items of 'tsd.http.request.enable_chunked = true'
	// and 'tsd.http.request.max_chunk = 40960' are all in the opentsdb.conf:
	DefaultMaxContentLength = 40960
)

// OpentsdbClient is the private implementation of the OpentsDBClient interface,
// which includes context-aware functionality.
type OpentsdbClient struct {
	tsdbEndpoint string
	client       *http.Client
	ctx          context.Context
	opentsdbCfg  OpenTSDBConfig
	logger       Logger
	metrics      Metrics
	tracer       trace.Tracer
}

// New initializes a new instance of Opentsdb with provided configuration.
func New(config *OpenTSDBConfig) OpentsdbProvider {
	return &OpentsdbClient{opentsdbCfg: *config}
}

// UseLogger sets the Logger interface for the FTP file system.
func (c *OpentsdbClient) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (c *OpentsdbClient) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Clickhouse client.
func (c *OpentsdbClient) UseTracer(tracer any) {
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
// If the configuration is invalid or the endpoint is unreachable, an error is returned.
func (c *OpentsdbClient) Connect() {
	c.opentsdbCfg.OpentsdbHost = strings.TrimSpace(c.opentsdbCfg.OpentsdbHost)
	if len(c.opentsdbCfg.OpentsdbHost) == 0 {
		c.logger.Errorf("the OpentsdbEndpoint in the given configuration cannot be empty.")
	}

	// Use custom transport settings if provided, otherwise, use the default transport.
	transport := c.opentsdbCfg.Transport
	if transport == nil {
		transport = DefaultTransport
	}

	c.client = &http.Client{
		Transport: transport,
	}

	// Set default values for optional configuration fields.
	if c.opentsdbCfg.MaxPutPointsNum <= 0 {
		c.opentsdbCfg.MaxPutPointsNum = DefaultMaxPutPointsNum
	}
	if c.opentsdbCfg.DetectDeltaNum <= 0 {
		c.opentsdbCfg.DetectDeltaNum = DefaultDetectDeltaNum
	}
	if c.opentsdbCfg.MaxContentLength <= 0 {
		c.opentsdbCfg.MaxContentLength = DefaultMaxContentLength
	}

	// Initialize the OpenTSDB client with the given configuration.
	c.tsdbEndpoint = fmt.Sprintf("http://%s", c.opentsdbCfg.OpentsdbHost)
}

// NewClientContext creates a new OpenTSDB client with context support.
// This allows the use of contexts for managing request timeouts or cancellations.
func NewClientContext(opentsdbCfg *OpenTSDBConfig) OpentsdbProviderWithContext {
	client := New(opentsdbCfg)

	return client.(OpentsdbProviderWithContext)
}

// WithContext creates a new OpenTSDB client that operates with the provided context.
func (c *OpentsdbClient) WithContext(ctx context.Context) OpentsDBClient {
	return &OpentsdbClient{
		tsdbEndpoint: c.tsdbEndpoint,
		client:       c.client,
		ctx:          ctx,
		opentsdbCfg:  c.opentsdbCfg,
	}
}

// sendRequest dispatches an HTTP request to the OpenTSDB server, using the provided
// method, URL, and body content. It returns the parsed response or an error, if any.
func (c *OpentsdbClient) sendRequest(method, url, reqBodyCnt string, parsedResp Response) error {
	// Create the HTTP request, attaching the context if available.
	req, err := http.NewRequest(method, url, strings.NewReader(reqBodyCnt))
	if c.ctx != nil {
		req = req.WithContext(c.ctx)
	}
	if err != nil {
		return fmt.Errorf("Failed to create request for %s %s: %v", method, url, err)
	}

	// Set the request headers.
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	// Send the request and handle the response.
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send request for %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	// Read and parse the response.
	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response for %s %s: %v", method, url, err)
	}

	parsedResp.SetStatus(resp.StatusCode)
	parser := parsedResp.GetCustomParser()
	if parser == nil {
		// Use the default JSON unmarshaller if no custom parser is provided.
		if err := json.Unmarshal(jsonBytes, parsedResp); err != nil {
			return fmt.Errorf("Failed to parse response for %s %s: %v", method, url, err)
		}
	} else {
		// Use the custom parser if available.
		if err := parser(jsonBytes); err != nil {
			return err
		}
	}

	return nil
}

// isValidOperateMethod checks if the provided HTTP method is valid for
// operations such as POST, PUT, or DELETE.
func (c *OpentsdbClient) isValidOperateMethod(method string) bool {
	method = strings.TrimSpace(strings.ToUpper(method))
	if len(method) == 0 {
		return false
	}
	validMethods := []string{PostMethod, PutMethod, DeleteMethod}
	for _, validMethod := range validMethods {
		if method == validMethod {
			return true
		}
	}
	return false
}

// Ping checks the availability of the OpenTSDB server by establishing a TCP connection.
func (c *OpentsdbClient) Ping() error {
	conn, err := net.DialTimeout("tcp", c.opentsdbCfg.OpentsdbHost, DefaultDialTime)
	if err != nil {
		return fmt.Errorf("The target OpenTSDB is unreachable: %v", err)
	}

	if conn != nil {
		defer conn.Close()
	}
	return nil
}
