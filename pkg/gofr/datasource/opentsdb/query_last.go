package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"strings"
	"time"
)

// QueryLastParam is the structure used to hold
// the querying parameters when calling /api/query/last.
// Each attributes in QueryLastParam matches the definition in
// (http://opentsdb.net/docs/build/html/api_http/query/last.html).
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

	logger Logger
	tracer trace.Tracer
}

func (query *QueryLastParam) String(ctx context.Context) string {
	_, span := query.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(query.logger, time.Now(), "ToString-QueryLastParam", &status, &message, span)

	content, err := json.Marshal(query)
	if err != nil {
		message = fmt.Sprintf("marshal queryLastParam error: %s", err)
		query.logger.Errorf(message)
	}

	status = "SUCCESS"
	message = fmt.Sprint("queryLastParam converted to string successfully")

	return string(content)
}

// SubQueryLast is the structure used to hold
// the subquery parameters when calling /api/query/last.
// Each attributes in SubQueryLast matches the definition in
// (http://opentsdb.net/docs/build/html/api_http/query/last.html).
type SubQueryLast struct {
	// The name of a metric stored in the system.
	// The value is reqiured with non-empty value.
	Metric string `json:"metric"`

	// An optional value to drill down to specific timeseries or group results by tag,
	// supply one or more map values in the same format as the query string. Tags are converted to filters in 2.2.
	// Note that if no tags are specified, all metrics in the system will be aggregated into the results.
	// It will be deprecated in OpenTSDB 2.2.
	Tags map[string]string `json:"tags,omitempty"`
}

// QueryLastResponse acts as the implementation of Response in the /api/query/last scene.
// It holds the status code and the response values defined in the
// (http://opentsdb.net/docs/build/html/api_http/query/last.html).
type QueryLastResponse struct {
	StatusCode    int
	QueryRespCnts []QueryRespLastItem    `json:"queryRespCnts,omitempty"`
	ErrorMsg      map[string]interface{} `json:"error"`
	logger        Logger
	tracer        trace.Tracer
}

func (queryLastResp *QueryLastResponse) String(ctx context.Context) string {
	_, span := queryLastResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(queryLastResp.logger, time.Now(), "ToString-QueryLastResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(queryLastResp)
	if err != nil {
		message = fmt.Sprintf("marshal queryLast response error: %s", err.Error())
		queryLastResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("query response converted to string successfully")

	return buffer.String()
}

func (queryLastResp *QueryLastResponse) SetStatus(ctx context.Context, code int) {
	_, span := queryLastResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(queryLastResp.logger, time.Now(), "SetStatus-QueryLastResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	queryLastResp.StatusCode = code
}

func (queryLastResp *QueryLastResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := queryLastResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(queryLastResp.logger, time.Now(), "GetCustomParser-QueryLastResp", &status, &message, span)

	return func(resp []byte) error {
		originRespStr := string(resp)
		var respStr string
		if queryLastResp.StatusCode == 200 && strings.Contains(originRespStr, "[") && strings.Contains(originRespStr, "]") {
			respStr = fmt.Sprintf("{%s:%s}", `"queryRespCnts"`, originRespStr)
		} else {
			respStr = originRespStr
		}

		err := json.Unmarshal([]byte(respStr), &queryLastResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal queryLast response error: %s", err)
			queryLastResp.logger.Errorf(message)

			return err
		}

		status = "SUCCESS"
		message = fmt.Sprint("queryLast custom parsing successful")

		return nil
	}
}

// QueryRespLastItem acts as the implementation of Response in the /api/query/last scene.
// It holds the response item defined in the
// (http://opentsdb.net/docs/build/html/api_http/query/last.html).
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
	Tsuid string `json:"tsuid"`
}

func (c *OpentsdbClient) QueryLast(param QueryLastParam) (*QueryLastResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "QueryLast")
	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryLast", &status, &message, span)

	if !isValidQueryLastParam(&param) {
		message = fmt.Sprintf("invalid query last param")
		return nil, errors.New(message)
	}
	queryEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, QueryLastPath)

	reqBodyCnt, err := getQueryBodyContents(&param)
	if err != nil {
		message = fmt.Sprint("error retrieving body contents: ", err)
		return nil, err
	}

	queryResp := QueryLastResponse{logger: c.logger, tracer: c.tracer}
	if err = c.sendRequest(PostMethod, queryEndpoint, reqBodyCnt, &queryResp); err != nil {
		message = fmt.Sprintf("error sending request at url %s : %s ", queryEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("querylast request to url %q processed successfully", queryEndpoint)
	c.logger.Logf("querylast request processed successfully")

	return &queryResp, nil
}

func isValidQueryLastParam(param *QueryLastParam) bool {
	if param.Queries == nil || len(param.Queries) == 0 {
		return false
	}
	for _, query := range param.Queries {
		if len(query.Metric) == 0 {
			return false
		}
	}
	return true
}
