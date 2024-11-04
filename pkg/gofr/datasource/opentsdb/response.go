package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/aggregators.html.
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

// VersionResponse is the struct implementation for /api/version.
type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]any
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

// Annotation holds parameters for querying or managing annotations via the /api/annotation endpoint in OpenTSDB.
// Used for logging notes on events at specific times, often tied to time series data, mainly for graphing or API queries.
type Annotation struct {
	// StartTime is the Unix epoch timestamp (in seconds) for when the event occurred. This is required.
	StartTime int64 `json:"startTime,omitempty"`

	// EndTime is the optional Unix epoch timestamp (in seconds) for when the event ended, if applicable.
	EndTime int64 `json:"endTime,omitempty"`

	// TSUID is the optional time series identifier if the annotation is linked to a specific time series.
	TSUID string `json:"tsuid,omitempty"`

	// Description is a brief, optional summary of the event (recommended to keep under 25 characters for display purposes).
	Description string `json:"description,omitempty"`

	// Notes is an optional, detailed description of the event.
	Notes string `json:"notes,omitempty"`

	// Custom is an optional key/value map to store any additional fields and their values.
	Custom map[string]string `json:"custom,omitempty"`
}

// AnnotationResponse encapsulates the response data and status when interacting with the /api/annotation endpoint.
type AnnotationResponse struct {

	// Annotation holds the associated annotation object.
	Annotation

	// ErrorInfo contains details about any errors that occurred during the request.
	ErrorInfo map[string]any `json:"error,omitempty"`

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

// QueryResponse acts as the implementation of Response in the /api/query scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type QueryResponse struct {
	QueryRespCnts []QueryRespItem `json:"queryRespCnts"`
	ErrorMsg      map[string]any  `json:"error"`
	logger        Logger
	tracer        trace.Tracer
	ctx           context.Context
}

// QueryRespItem acts as the implementation of Response in the /api/query scene.
// It holds the response item defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type QueryRespItem struct {
	// Name of the metric retrieved for the time series
	Metric string `json:"metric"`

	// A list of tags only returned when the results are for a single time series.
	// If results are aggregated, this value may be null or an empty map
	Tags map[string]string `json:"tags"`

	// If more than one time series were included in the result set, i.e. they were aggregated,
	// this will display a list of tag names that were found in common across all time series.
	// Note that: Api Doc uses 'aggregatedTags', but actual response uses 'aggregateTags'
	AggregatedTags []string `json:"aggregateTags"`

	// Retrieved data points after being processed by the aggregators. Each data point consists
	// of a timestamp and a value, the format determined by the serializer.
	// For the JSON serializer, the timestamp will always be a Unix epoch style integer followed
	// by the value as an integer or a floating point.
	// For example, the default output is "dps"{"<timestamp>":<value>}.
	// By default, the timestamps will be in seconds. If the msResolution flag is set, then the
	// timestamps will be in milliseconds.
	//
	// Because the elements of map is out of order, using common way to iterate Dps will not get
	// data points with timestamps out of order.
	// So be aware that one should use '(qri *QueryRespItem) GetDataPoints() []*DataPoint' to
	// acquire the real ascending data points.
	Dps map[string]any `json:"dps"`

	// If the query retrieved annotations for time series over the requested timespan, they will
	// be returned in this group. Annotations for every time series will be merged into one set
	// and sorted by start_time. Aggregator functions do not affect annotations, all annotations
	// will be returned for the span.
	// The value is optional.
	Annotations []Annotation `json:"annotations,omitempty"`

	// If requested by the user, the query will scan for global annotations during
	// the timespan and the results returned in this group.
	// The value is optional.
	GlobalAnnotations []Annotation `json:"globalAnnotations,omitempty"`

	tracer trace.Tracer
}

// QueryLastResponse acts as the implementation of Response in the /api/query/last scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type QueryLastResponse struct {
	QueryRespCnts []QueryRespLastItem `json:"queryRespCnts,omitempty"`
	ErrorMsg      map[string]any      `json:"error"`
	logger        Logger
	tracer        trace.Tracer
	ctx           context.Context
}

// QueryRespLastItem acts as the implementation of Response in the /api/query/last scene.
// It holds the response item defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type QueryRespLastItem struct {
	// Name of the metric retreived for the time series.
	// Only returned if resolve was set to true.
	Metric string `json:"metric"`

	// A list of tags only returned when the results are for a single time series.
	// If results are aggregated, this value may be null or an empty map.
	// Only returned if resolve was set to true.
	Tags map[string]string `json:"tags"`

	// A Unix epoch timestamp, in milliseconds, when the data point was written.
	Timestamp int64 `json:"timestamp"`

	// The value of the data point enclosed in quotation marks as a string
	Value string `json:"value"`

	// The hexadecimal TSUID for the time series
	TSUID string `json:"tsuid"`
}

func (*PutResponse) getCustomParser() func(respCnt []byte) error {
	return nil
}

func (queryResp *QueryResponse) getCustomParser() func(respCnt []byte) error {
	return queryParserHelper(queryResp.ctx, queryResp.logger, queryResp, "GetCustomParser-Query")
}

func (queryLastResp *QueryLastResponse) getCustomParser() func(respCnt []byte) error {
	return queryParserHelper(queryLastResp.ctx, queryLastResp.logger, queryLastResp, "GetCustomParser-QueryLast")
}

func (verResp *VersionResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(verResp.ctx, verResp, "GetCustomParser-VersionResp", verResp.logger,
		func(resp []byte) error {
			v := make(map[string]any, 0)

			err := json.Unmarshal(resp, &v)
			if err != nil {
				return err
			}

			verResp.VersionInfo = v

			return nil
		})
}

func (annotResp *AnnotationResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(annotResp.ctx, annotResp, "getCustomParser-Annotation", annotResp.logger,
		func(resp []byte) error {
			if len(resp) == 0 {
				return nil
			}

			return json.Unmarshal(resp, &annotResp)
		})
}

func (aggreResp *AggregatorsResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(aggreResp.ctx, aggreResp, "GetCustomParser-Aggregator", aggreResp.logger,
		func(resp []byte) error {
			j := make([]string, 0)

			err := json.Unmarshal(resp, &j)
			if err != nil {
				return err
			}

			aggreResp.Aggregators = j

			return nil
		})
}

type genericResponse interface {
	addTrace(ctx context.Context, operation string) trace.Span
}

// sendRequest dispatches an HTTP request to the OpenTSDB server, using the provided
// method, URL, and body content. It returns the parsed response or an error, if any.
func (c *Client) sendRequest(ctx context.Context, method, url, reqBodyCnt string, parsedResp Response) error {
	span := c.addTrace(ctx, "sendRequest")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "sendRequest", &status, &message, span)

	// Create the HTTP request, attaching the context if available.
	req, err := http.NewRequest(method, url, strings.NewReader(reqBodyCnt))
	if ctx != nil {
		req = req.WithContext(ctx)
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

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("client error: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	// Read and parse the response.
	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		errReading := fmt.Errorf("failed to read response body for %s %s: %w", method, url, err)

		message = fmt.Sprint(errReading)

		return errReading
	}

	parser := parsedResp.getCustomParser()
	if parser == nil {
		// Use the default JSON unmarshaller if no custom parser is provided.
		if err = json.Unmarshal(jsonBytes, parsedResp); err != nil {
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

func (c *Client) version(ctx context.Context, verResp *VersionResponse) error {
	span := c.addTrace(ctx, "Version")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.endpoint, VersionPath)
	verResp.logger = c.logger
	verResp.tracer = c.tracer
	verResp.ctx = ctx

	if err := c.sendRequest(ctx, http.MethodGet, verEndpoint, "", verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return err
	}

	status = StatusSuccess
	message = "version response retrieved successfully."

	return nil
}

// isValidOperateMethod checks if the provided HTTP method is valid for
// operations such as POST, PUT, or DELETE.
func (c *Client) isValidOperateMethod(ctx context.Context, method string) bool {
	span := c.addTrace(ctx, "isValidOperateMethod")

	status := StatusSuccess

	var message string

	defer sendOperationStats(c.logger, time.Now(), "isValidOperateMethod", &status, &message, span)

	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return false
	}

	validMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPut}
	for _, validMethod := range validMethods {
		if method == validMethod {
			return true
		}
	}

	return false
}

func customParserHelper(ctx context.Context, resp genericResponse, operation string, logger Logger,
	unmarshalFunc func([]byte) error) func([]byte) error {
	span := resp.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)

	return func(result []byte) error {
		err := unmarshalFunc(result)
		if err != nil {
			message = fmt.Sprintf("unmarshal %s error: %s", operation, err)
			logger.Errorf(message)

			return err
		}

		status = StatusSuccess
		message = fmt.Sprintf("%s custom parsing was successful.", operation)

		return nil
	}
}

func queryParserHelper(ctx context.Context, logger Logger, obj genericResponse, methodName string) func(respCnt []byte) error {
	return customParserHelper(ctx, obj, methodName, logger, func(resp []byte) error {
		originRespStr := string(resp)

		var respStr string

		if len(resp) != 0 && resp[0] == '[' && resp[len(resp)-1] == ']' {
			respStr = fmt.Sprintf(`{"queryRespCnts":%s}`, originRespStr)
		} else {
			respStr = originRespStr
		}

		return json.Unmarshal([]byte(respStr), obj)
	})
}

func (c *Client) operateAnnotation(ctx context.Context, queryAnnotation, resp any, method, operation string) error {
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
