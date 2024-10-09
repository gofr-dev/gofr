package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
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
	return toString(ctx, query, "ToString-QueryLastParam", query.logger)
}

func (query *QueryLastParam) setStatusCode(int) {
	query.logger.Errorf("method not supported by opentsdb-client")
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
	ctx           context.Context
}

func (queryLastResp *QueryLastResponse) String() string {
	return toString(queryLastResp.ctx, queryLastResp, "ToString-QueryLast", queryLastResp.logger)
}

func (queryLastResp *QueryLastResponse) SetStatus(code int) {
	setStatus(queryLastResp.ctx, queryLastResp, code, "SetStatus-QueryLast", queryLastResp.logger)
}

func (queryLastResp *QueryLastResponse) setStatusCode(code int) {
	queryLastResp.StatusCode = code
}

func (queryLastResp *QueryLastResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(queryLastResp.ctx, queryLastResp, "GetCustomParser-QueryLast", queryLastResp.logger, func(resp []byte) error {
		originRespStr := string(resp)

		var respStr string

		if queryLastResp.StatusCode == http.StatusOK && strings.Contains(originRespStr, "[") && strings.Contains(originRespStr, "]") {
			respStr = fmt.Sprintf("{%s:%s}", `"queryRespCnts"`, originRespStr)
		} else {
			respStr = originRespStr
		}

		return json.Unmarshal([]byte(respStr), &queryLastResp)
	})
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

func (c *OpentsdbClient) QueryLast(param *QueryLastParam) (*QueryLastResponse, error) {
	span := c.addTrace(c.ctx, "QueryLast")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryLast", &status, &message, span)

	if !isValidQueryLastParam(param) {
		message = "invalid query last param"
		return nil, errors.New(message)
	}

	queryEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, QueryLastPath)

	reqBodyCnt, err := getQueryBodyContents(param)
	if err != nil {
		message = fmt.Sprint("error retrieving body contents: ", err)
		return nil, err
	}

	queryResp := QueryLastResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}
	if err = c.sendRequest(PostMethod, queryEndpoint, reqBodyCnt, &queryResp); err != nil {
		message = fmt.Sprintf("error sending request at url %s : %s ", queryEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("querylast request to url %q processed successfully", queryEndpoint)

	c.logger.Logf("querylast request processed successfully")

	return &queryResp, nil
}

func isValidQueryLastParam(param *QueryLastParam) bool {
	if param.Queries == nil || len(param.Queries) == 0 {
		return false
	}

	for _, query := range param.Queries {
		if query.Metric == "" {
			return false
		}
	}
	return true
}
