package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var errInvalidDataPoint = errors.New("invalid data points")

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
}

// SubQuery is the structure used to hold the subquery parameters when calling /api/query.
// Each attributes in SubQuery matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/index.html.
type SubQuery struct {
	// The name of an aggregation function to use.
	// The value is required with non-empty one in the range of
	// the response of calling /api/aggregators.
	//
	// By default, the potential values and corresponding descriptions are as following:
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
	// queryRateOptionCounter (value type is bool),  queryRateOptionCounterMax (value type is int,int64)
	// queryRateOptionResetValue (value type is int,int64)
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

// DataPoint is the structure used to hold the values of a metric item. Each attributes
// in DataPoint matches the definition in [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/put.html.
type DataPoint struct {
	// The name of the metric which is about to be stored, and is required with non-empty value.
	Metric string `json:"metric"`

	// A Unix epoch style timestamp in seconds or milliseconds.
	// The timestamp must not contain non-numeric characters.
	// One can use time.Now().Unix() to set this attribute.
	// This attribute is also required with non-zero value.
	Timestamp int64 `json:"timestamp"`

	// The real type of Value only could be int, int64, float64, or string, and is required.
	Value any `json:"value"`

	// A map of tag name/tag value pairs. At least one pair must be supplied.
	// Don't use too many tags, keep it to a fairly small number, usually up to 4 or 5 tags
	// (By default, OpenTSDB supports a maximum of 8 tags, which can be modified by add
	// configuration item 'tsd.storage.max_tags' in opentsdb.conf).
	Tags map[string]string `json:"tags"`
}

// PutError holds the error message for each putting DataPoint instance. Only calling PUT() with "details"
// query parameter, the response of the failed put data operation can contain an array PutError instance
// to show the details for each failure.
type PutError struct {
	Data     DataPoint `json:"datapoint"`
	ErrorMsg string    `json:"error"`
}

// PutResponse acts as the implementation of Response in the /api/put scene.
// It holds the status code and the response values defined in
// the [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/put.html.
type PutResponse struct {
	Failed  int64      `json:"failed"`
	Success int64      `json:"success"`
	Errors  []PutError `json:"errors,omitempty"`
}

func (c *Client) getResponse(ctx context.Context, putEndpoint string, datapoints []DataPoint,
	message *string) (*PutResponse, error) {
	marshaled, err := json.Marshal(datapoints)
	if err != nil {
		*message = fmt.Sprintf("getPutBodyContents error: %s", err)
		c.logger.Errorf(*message)
	}

	reqBodyCnt := string(marshaled)

	putResp := PutResponse{}

	if err = c.sendRequest(ctx, http.MethodPost, putEndpoint, reqBodyCnt, &putResp); err != nil {
		*message = fmt.Sprintf("error processing Put request at url %q: %s", putEndpoint, err)
		return nil, err
	}

	return &putResp, nil
}

func parsePutErrorMsg(resp *PutResponse) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Failed to put %d datapoint(s) into opentsdb \n", resp.Failed))

	if len(resp.Errors) > 0 {
		for _, putError := range resp.Errors {
			str, err := json.Marshal(putError)
			if err != nil {
				return err
			}

			buf.WriteString(fmt.Sprintf("\t%s\n", str))
		}
	}

	return fmt.Errorf("%w: %s", errUnexpected, buf.String())
}

func validateDataPoint(datas []DataPoint) error {
	if len(datas) == 0 {
		return fmt.Errorf("%w: datapoints are empty", errInvalidDataPoint)
	}

	for _, data := range datas {
		if !isValidDataPoint(&data) {
			return fmt.Errorf("%w: please give a valid value", errInvalidDataPoint)
		}
	}

	return nil
}

func isValidDataPoint(data *DataPoint) bool {
	if data.Metric == "" || data.Timestamp == 0 || len(data.Tags) < 1 || data.Value == nil {
		return false
	}

	switch data.Value.(type) {
	case int64:
	case int:
	case float64:
	case float32:
	case string:
	default:
		return false
	}

	return true
}

func isValidPutParam(param string) bool {
	if isEmptyPutParam(param) {
		return true
	}

	param = strings.TrimSpace(param)
	if param != putRespWithSummary && param != putRespWithDetails {
		return false
	}

	return true
}

func isEmptyPutParam(param string) bool {
	return strings.TrimSpace(param) == ""
}

// QueryLastParam is the structure used to hold
// the querying parameters when calling /api/query/last.
// Each attributes in QueryLastParam matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type QueryLastParam struct {
	// One or more sub queries used to select the time series to return.
	// These may be metric m or TSUID tsuids queries
	// The value is required with at least one element
	Queries []SubQueryLast `json:"queries"`

	// An optional flag is used to determine whether or not to resolve the TSUIDs of results to
	// their metric and tag names. The default value is false.
	ResolveNames bool `json:"resolveNames"`

	// An optional number of hours is used to search in the past for data. If set to 0 then the
	// timestamp of the meta data counter for the time series is used.
	BackScan int `json:"backScan"`
}

// SubQueryLast is the structure used to hold the subquery parameters when calling /api/query/last.
// Each attributes in SubQueryLast matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/query/last.html.
type SubQueryLast struct {
	// The name of a metric stored in the system.
	// The value is required with non-empty value.
	Metric string `json:"metric"`

	// An optional value to drill down to specific time series or group results by tag,
	// supply one or more map values in the same format as the query string. Tags are converted to filters in 2.2.
	// Note that if no tags are specified, all metrics in the system will be aggregated into the results.
	// It will be deprecated in OpenTSDB 2.2.
	Tags map[string]string `json:"tags,omitempty"`
}

func getQueryBodyContents(param any) (string, error) {
	result, err := json.Marshal(param)
	if err != nil {
		return "", fmt.Errorf("failed to marshal query param: %w", err)
	}

	return string(result), nil
}

func isValidQueryParam(param *QueryParam) bool {
	if len(param.Queries) == 0 {
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
		if k != queryRateOptionCounter && k != queryRateOptionCounterMax && k != queryRateOptionResetValue {
			return false
		}
	}

	return true
}

func isValidTimePoint(timePoint any) bool {
	if timePoint == nil {
		return false
	}

	switch v := timePoint.(type) {
	case int:
		return v > 0
	case int64:
		return v > 0
	case string:
		return v != ""
	}

	return false
}

func isValidQueryLastParam(param *QueryLastParam) bool {
	if len(param.Queries) == 0 {
		return false
	}

	for _, query := range param.Queries {
		if query.Metric == "" {
			return false
		}
	}

	return true
}

func (c *Client) initializeClient() {
	// defaultTransport defines the default HTTP transport settings,
	// including connection timeouts and idle connections.
	var defaultTransport = &http.Transport{
		MaxIdleConnsPerHost: 10,
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTime,
			KeepAlive: connectionTimeout,
		}).DialContext,
	}

	c.config.Host = strings.TrimSpace(c.config.Host)
	if c.config.Host == "" {
		c.logger.Fatal("the OpenTSDB Endpoint in the given configuration cannot be empty.")
	}

	// Use custom transport settings if provided, otherwise, use the default transport.
	transport := c.config.Transport
	if transport == nil {
		transport = defaultTransport
	}

	c.client = &http.Client{
		Transport: transport,
	}

	if c.config.MaxPutPointsNum <= 0 {
		c.config.MaxPutPointsNum = defaultMaxPutPointsNum
	}

	if c.config.DetectDeltaNum <= 0 {
		c.config.DetectDeltaNum = defaultDetectDeltaNum
	}

	if c.config.MaxContentLength <= 0 {
		c.config.MaxContentLength = defaultMaxContentLength
	}
}
