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

var (
	errResp            = errors.New("error response from OpenTSDB server")
	errInvalidArgument = errors.New("invalid argument type")
	errUnexpected      = errors.New("unexpected error")
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/aggregators.html.
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string
}

// VersionResponse is the struct implementation for /api/version.
type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]any
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
}

// QueryResponse acts as the implementation of Response in the /api/query scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type QueryResponse struct {
	QueryRespCnts []QueryRespItem `json:"queryRespCnts"`
	ErrorMsg      map[string]any  `json:"error"`
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
}

// QueryLastResponse acts as the implementation of Response in the /api/query/last scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type QueryLastResponse struct {
	QueryRespCnts []QueryRespLastItem `json:"queryRespCnts,omitempty"`
	ErrorMsg      map[string]any      `json:"error"`
}

// QueryRespLastItem acts as the implementation of Response in the /api/query/last scene.
// It holds the response item defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type QueryRespLastItem struct {
	// Name of the metric retrieved for the time series.
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

func (*PutResponse) getCustomParser(Logger) func(respCnt []byte) error {
	return nil
}

func (queryResp *QueryResponse) getCustomParser(logger Logger) func(respCnt []byte) error {
	return queryParserHelper(logger, queryResp, "GetCustomParser-Query")
}

func (queryLastResp *QueryLastResponse) getCustomParser(logger Logger) func(respCnt []byte) error {
	return queryParserHelper(logger, queryLastResp, "GetCustomParser-QueryLast")
}

func (verResp *VersionResponse) getCustomParser(logger Logger) func(respCnt []byte) error {
	return customParserHelper("GetCustomParser-VersionResp", logger,
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

func (annotResp *AnnotationResponse) getCustomParser(logger Logger) func(respCnt []byte) error {
	return customParserHelper("getCustomParser-Annotation", logger,
		func(resp []byte) error {
			if len(resp) == 0 {
				return nil
			}

			return json.Unmarshal(resp, &annotResp)
		})
}

func (aggreResp *AggregatorsResponse) getCustomParser(logger Logger) func(respCnt []byte) error {
	return customParserHelper("GetCustomParser-Aggregator", logger,
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
	addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span
}

// sendRequest dispatches an HTTP request to the OpenTSDB server, using the provided
// method, URL, and body content. It returns the parsed response or an error, if any.
func (c *Client) sendRequest(ctx context.Context, method, url, reqBodyCnt string, parsedResp response) error {
	// Create the HTTP request, attaching the context if available.
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(reqBodyCnt))
	if ctx != nil {
		req = req.WithContext(ctx)
	}

	if err != nil {
		errRequestCreation := fmt.Errorf("failed to create request for %s %s: %w", method, url, err)

		return errRequestCreation
	}

	// Set the request headers.
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	// Send the request and handle the response.
	resp, err := c.client.Do(req)
	if err != nil {
		sendRequestErr := fmt.Errorf("failed to send request for %s %s: %w", method, url, err)

		return sendRequestErr
	}

	defer resp.Body.Close()

	// Read and parse the response.
	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		errReading := fmt.Errorf("failed to read response body for %s %s: %w", method, url, err)

		return errReading
	}

	parser := parsedResp.getCustomParser(c.logger)
	if parser == nil {
		// Use the default JSON unmarshaller if no custom parser is provided.
		if err = json.Unmarshal(jsonBytes, parsedResp); err != nil {
			errUnmarshalling := fmt.Errorf("failed to unmarshal response body for %s %s: %w", method, url, err)

			return errUnmarshalling
		}
	} else {
		// Use the custom parser if available.
		if err := parser(jsonBytes); err != nil {
			return fmt.Errorf("failed to parse response body through custom parser %s %s: %w", method, url, err)
		}
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("%w, status code: %d", errResp, resp.StatusCode)
	}

	return nil
}

func (c *Client) version(ctx context.Context, verResp *VersionResponse) error {
	span := c.addTrace(ctx, "Version")

	status := statusFailed

	message := "version request failed"

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.endpoint, versionPath)

	if err := c.sendRequest(ctx, http.MethodGet, verEndpoint, "", verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("OpenTSDB version %v", verResp.VersionInfo["version"])

	return nil
}

// isValidOperateMethod checks if the provided HTTP method is valid for
// operations such as POST, PUT, or DELETE.
func (*Client) isValidOperateMethod(method string) bool {
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

func customParserHelper(operation string, logger Logger, unmarshalFunc func([]byte) error) func([]byte) error {
	return func(result []byte) error {
		err := unmarshalFunc(result)
		if err != nil {
			logger.Errorf("unmarshal %s error: %s", operation, err)

			return err
		}

		return nil
	}
}

func queryParserHelper(logger Logger, obj genericResponse,
	methodName string) func(respCnt []byte) error {
	return customParserHelper(methodName, logger, func(resp []byte) error {
		originRespStr := string(resp)

		var respStr string

		if strings.HasPrefix(string(resp), "[") && strings.HasSuffix(string(resp), "]") {
			respStr = fmt.Sprintf(`{"queryRespCnts":%s}`, originRespStr)
		} else {
			respStr = originRespStr
		}

		return json.Unmarshal([]byte(respStr), obj)
	})
}

func (c *Client) operateAnnotation(ctx context.Context, queryAnnotation, resp any, method, operation string) error {
	span := c.addTrace(ctx, operation)

	status := statusFailed

	message := fmt.Sprintf("%v request failed", operation)

	defer sendOperationStats(ctx, c.logger, c.metrics, c.config.Host, time.Now(), operation, &status, &message, span)

	annotation, ok := queryAnnotation.(*Annotation)
	if !ok {
		return fmt.Errorf("%w: Must be *Annotation", errInvalidArgument)
	}

	annResp, ok := resp.(*AnnotationResponse)
	if !ok {
		return fmt.Errorf("%w: Must be *AnnotationResponse", errInvalidResponseType)
	}

	if !c.isValidOperateMethod(method) {
		message = fmt.Sprintf("invalid annotation operation method: %s", method)
		return fmt.Errorf("%w: %s", errUnexpected, message)
	}

	annoEndpoint := fmt.Sprintf("%s%s", c.endpoint, annotationPath)

	resultBytes, err := json.Marshal(annotation)
	if err != nil {
		message = fmt.Sprintf("marshal annotation response error: %s", err)
		return fmt.Errorf("%w: %s", errUnexpected, message)
	}

	if err = c.sendRequest(ctx, method, annoEndpoint, string(resultBytes), annResp); err != nil {
		message = fmt.Sprintf("error processing %s annotation request to url %q: %s", method, annoEndpoint, err.Error())
		return err
	}

	status = statusSuccess
	message = fmt.Sprintf("%s: %s annotation request to url %q processed successfully", operation, method, annoEndpoint)

	c.logger.Log("%s request successful", operation)

	return nil
}
