package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// QueryParam is the structure used to hold the querying parameters when calling /api/query.
// Each attributes in QueryParam matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type QueryParam struct {
	// The start time for the query. This can be a relative or absolute timestamp.
	// The data type can only be string, int, or int64.
	// The value is required with non-zero value of the target type.
	Start any `json:"start"`

	// An end time for the query. If not supplied, the TSD will assume the local
	// system time on the server. This may be a relative or absolute timestamp.
	// The data type can only be string, or int64.
	// The value is optional.
	End any `json:"end,omitempty"`

	// One or more sub queries used to select the time series to return.
	// These may be metric m or TSUID tsuids queries
	// The value is required with at least one element
	Queries []SubQuery `json:"queries"`

	// An optional value is used to show whether to return annotations with a query.
	// The default is to return annotations for the requested timespan but this flag can disable the return.
	// This affects both local and global notes and overrides globalAnnotations
	NoAnnotations bool `json:"noAnnotations,omitempty"`

	// An optional value is used to show whether the query should retrieve global
	// annotations for the requested timespan.
	GlobalAnnotations bool `json:"globalAnnotations,omitempty"`

	// An optional value is used to show whether to output data point timestamps in milliseconds or seconds.
	// If this flag is not provided and there are multiple data points within a second,
	// those data points will be down sampled using the query's aggregation function.
	MsResolution bool `json:"msResolution,omitempty"`

	// An optional value is used to show whether to output the TSUIDs associated with time series in the results.
	// If multiple time series were aggregated into one set, multiple TSUIDs will be returned in a sorted manner.
	ShowTSUIDs bool `json:"showTSUIDs,omitempty"`

	// An optional value is used to show whether can be passed to the JSON with a POST to delete any data point
	// that match the given query.
	Delete bool `json:"delete,omitempty"`

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

func (query *QueryParam) String() string {
	return toString(query.ctx, query, "ToString-QueryParam", query.logger)
}

func (*QueryParam) setStatusCode(int) {
	// method not implemented
}

// SubQuery is the structure used to hold the subquery parameters when calling /api/query.
// Each attributes in SubQuery matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type SubQuery struct {
	// The name of an aggregation function to use.
	// The value is required with non-empty one in the range of
	// the response of calling /api/aggregators.
	//
	// By default, the potential values and corresponding descriptions are as followings:
	//   "sum": Adds all of the data points for a timestamp.
	//   "min": Picks the smallest data point for each timestamp.
	//   "max": Picks the largest data point for each timestamp.
	//   "avg": Averages the values for the data points at each timestamp.
	Aggregator string `json:"aggregator"`

	// The name of a metric stored in the system.
	// The value is required with non-empty value.
	Metric string `json:"metric"`

	// An optional value is used to show whether the data should be
	// converted into deltas before returning. This is useful if the metric is a
	// continuously incrementing counter, and you want to view the rate of change between data points.
	Rate bool `json:"rate,omitempty"`

	// rateOptions represents monotonically increasing counter handling options.
	// The value is optional.
	// Currently, there is only three kind of value can be set to this map:
	// Only three keys can be set into the rateOption parameter of the QueryParam is
	// QueryRateOptionCounter (value type is bool),  QueryRateOptionCounterMax (value type is int,int64)
	// QueryRateOptionResetValue (value type is int,int64)
	RateParams map[string]any `json:"rateOptions,omitempty"`

	// An optional value downsampling function to reduce the amount of data returned.
	DownSample string `json:"downsample,omitempty"`

	// An optional value to drill down to specific time series or group results by tag,
	// supply one or more map values in the same format as the query string. Tags are converted to filters in 2.2.
	// Note that if no tags are specified, all metrics in the system will be aggregated into the results.
	// It will be deprecated in OpenTSDB 2.2.
	Tags map[string]string `json:"tags,omitempty"`

	// An optional value used to filter the time series emitted in the results.
	// Note that if no filters are specified, all time series for the given
	// metric will be aggregated into the results.
	Fiters []Filter `json:"filters,omitempty"`
}

// Filter is the structure used to hold the filter parameters when calling /api/query.
// Each attributes in Filter matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type Filter struct {
	// The name of the filter to invoke. The value is required with a non-empty
	// value in the range of calling /api/config/filters.
	Type string `json:"type"`

	// The tag key to invoke the filter on, required with a non-empty value
	Tagk string `json:"tagk"`

	// The filter expression to evaluate and depends on the filter being used, required with a non-empty value
	FilterExp string `json:"filter"`

	// An optional value to show whether to group the results by each value matched by the filter.
	// By default, all values matching the filter will be aggregated into a single series.
	GroupBy bool `json:"groupBy"`
}

// QueryResponse acts as the implementation of Response in the /api/query scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type QueryResponse struct {
	StatusCode    int
	QueryRespCnts []QueryRespItem `json:"queryRespCnts"`
	ErrorMsg      map[string]any  `json:"error"`
	logger        Logger
	tracer        trace.Tracer
	ctx           context.Context
}

func (queryResp *QueryResponse) String() string {
	return toString(queryResp.ctx, queryResp, "ToString-Query", queryResp.logger)
}

func (queryResp *QueryResponse) SetStatus(code int) {
	setStatus(queryResp.ctx, queryResp, code, "SetStatus-Query", queryResp.logger)
}

func (queryResp *QueryResponse) setStatusCode(code int) {
	queryResp.StatusCode = code
}

func (queryResp *QueryResponse) GetCustomParser() func(respCnt []byte) error {
	return getQueryParser(queryResp.ctx, queryResp.StatusCode, queryResp.logger, queryResp, "GetCustomParser-Query")
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

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

// GetDataPoints returns the real ascending data points from the information of the related QueryRespItem.
func (qri *QueryRespItem) GetDataPoints() []*DataPoint {
	span := qri.addTrace(qri.ctx, "GetDataPoints-QueryRespItem")

	status := StatusFailed

	var message string

	defer sendOperationStats(qri.logger, time.Now(), "GetDataPoints-QueryRespItem", &status, &message, span)

	datapoints := make([]*DataPoint, 0)

	timestampStrs := qri.getSortedTimestampStrs()
	for _, timestampStr := range timestampStrs {
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			message = fmt.Sprintf("parse timestamp error: %s", err)
			qri.logger.Errorf(message)
		}

		datapoint := &DataPoint{
			Metric:    qri.Metric,
			Value:     qri.Dps[timestampStr],
			Tags:      qri.Tags,
			Timestamp: timestamp,
		}
		datapoints = append(datapoints, datapoint)
	}

	status = StatusSuccess
	message = "dataPoints fetched successfully"

	return datapoints
}

// getSortedTimestampStrs returns a slice of the ascending timestamp with
// string format for the Dps of the related QueryRespItem instance.
func (qri *QueryRespItem) getSortedTimestampStrs() []string {
	span := qri.addTrace(qri.ctx, "GetSortedTimeStamps-QueryRespItem")

	status := StatusSuccess

	var message string

	defer sendOperationStats(qri.logger, time.Now(), "GetSortedTimeStamps-QueryRespItem", &status, &message, span)

	timestampStrs := make([]string, 0)
	for timestampStr := range qri.Dps {
		timestampStrs = append(timestampStrs, timestampStr)
	}

	sort.Strings(timestampStrs)

	return timestampStrs
}

// GetLatestDataPoint returns latest datapoint for the related QueryRespItem instance.
func (qri *QueryRespItem) GetLatestDataPoint() *DataPoint {
	span := qri.addTrace(qri.ctx, "GetLatestDataPoint-QueryRespItem")

	status := StatusFailed

	var message string

	defer sendOperationStats(qri.logger, time.Now(), "GetLatestDataPoint-QueryRespItem", &status, &message, span)

	timestampStrs := qri.getSortedTimestampStrs()

	size := len(timestampStrs)
	if size == 0 {
		message = "No datapoints present"
		return nil
	}

	timestamp, err := strconv.ParseInt(timestampStrs[size-1], 10, 64)
	if err != nil {
		message = fmt.Sprintf("parse timestamp error: %s", err)
		qri.logger.Errorf(message)
	}

	datapoint := &DataPoint{
		Metric:    qri.Metric,
		Value:     qri.Dps[timestampStrs[size-1]],
		Tags:      qri.Tags,
		Timestamp: timestamp,
	}

	status = StatusSuccess
	message = fmt.Sprintf("LatestDataPoints with timestamp %v fetched successfully", timestamp)

	qri.logger.Log("LatestDataPoints fetched successfully")

	return datapoint
}

func (c *Client) Query(param *QueryParam) (*QueryResponse, error) {
	if param.tracer == nil {
		param.tracer = c.tracer
	}

	if param.logger == nil {
		param.logger = c.logger
	}

	span := c.addTrace(c.ctx, "Query")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Query", &status, &message, span)

	if !isValidQueryParam(param) {
		message = "invalid query parameters"
		return nil, errors.New(message)
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.endpoint, QueryPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprintf("getQueryBodyContents error: %s", err)
		return nil, err
	}

	queryResp := QueryResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err = c.sendRequest(http.MethodPost, queryEndpoint, reqBodyCnt, &queryResp); err != nil {
		message = fmt.Sprintf("error while processing request at url %q: %s ", queryEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("query request at url %q processed successfully", queryEndpoint)

	return &queryResp, nil
}

func getQueryBodyContents(param any) (string, error) {
	result, err := json.Marshal(param)
	if err != nil {
		return "", fmt.Errorf("failed to marshal query param: %v", err)
	}

	return string(result), nil
}

func isValidQueryParam(param *QueryParam) bool {
	if param.Queries == nil || len(param.Queries) == 0 {
		return false
	}

	if !isValidTimePoint(param.Start) {
		return false
	}

	for _, query := range param.Queries {
		if !areValidParams(&query) {
			return false
		}
	}

	return true
}

func areValidParams(query *SubQuery) bool {
	if query.Aggregator == "" || query.Metric == "" {
		return false
	}

	for k := range query.RateParams {
		if k != QueryRateOptionCounter && k != QueryRateOptionCounterMax && k != QueryRateOptionResetValue {
			return false
		}
	}

	return true
}

func isValidTimePoint(timePoint interface{}) bool {
	if timePoint == nil {
		return false
	}

	switch v := timePoint.(type) {
	case int:
		if v <= 0 {
			return false
		}
	case int64:
		if v <= 0 {
			return false
		}
	case string:
		if v == "" {
			return false
		}

	default:
		return false
	}

	return true
}
