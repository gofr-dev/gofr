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

// UIDMetaData is the structure used to hold
// the parameters when calling (POST,PUT) on /api/uid/uidmeta.
// Each attributes in UIDMetaData matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/uid/uidmeta.html.
type UIDMetaData struct {
	// A required hexadecimal representation of the UID
	UID string `json:"uid,omitempty"`

	// A required type of UID, must be metric, tagk or tagv
	Type string `json:"type,omitempty"`

	// An optional brief description of what the UID represents
	Description string `json:"description,omitempty"`

	// An optional short name that can be displayed in GUIs instead of the default name
	DisplayName string `json:"displayName,omitempty"`

	// An optional detailed notes about what the UID represents
	Notes string `json:"notes,omitempty"`

	// An optional key/value map to store custom fields and values
	Custom map[string]string `json:"custom,omitempty"`
}

// UIDMetaDataResponse acts as the implementation of Response in the /api/uid/uidmeta scene.
// It holds the status code and the response values defined in the
// (http://opentsdb.net/docs/build/html/api_http/uid/uidmeta.html).
type UIDMetaDataResponse struct {
	UIDMetaData

	StatusCode int

	// The name of the UID as given when the data point was stored or the UID assigned
	Name string `json:"name,omitempty"`

	// A Unix epoch timestamp in seconds when the UID was first created.
	// If the meta data was not stored when the UID was assigned, this value may be 0.
	Created int64 `json:"created,omitempty"`

	ErrorInfo map[string]interface{} `json:"error,omitempty"`

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

func isValidUIDMetaDataQueryParam(metaQueryParam map[string]string) bool {
	if metaQueryParam == nil || len(metaQueryParam) != 2 {
		return false
	}

	checkKeys := []string{"uid", "type"}
	for _, checkKey := range checkKeys {
		_, exists := metaQueryParam[checkKey]
		if !exists {
			return false
		}
	}

	typeValue := metaQueryParam["type"]

	typeCheckItems := []string{TypeMetrics, TypeTagk, TypeTagv}

	for _, checkItem := range typeCheckItems {
		if typeValue == checkItem {
			return true
		}
	}

	return false
}

func (c *Client) QueryUIDMetaData(metaQueryParam map[string]string) (*UIDMetaDataResponse, error) {
	span := c.addTrace(c.ctx, "QueryUIDMetaData")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryUIDMetaData", &status, &message, span)

	if !isValidUIDMetaDataQueryParam(metaQueryParam) {
		message = "given query uid metadata is invalid"
		return nil, errors.New(message)
	}

	queryParam := fmt.Sprintf("%s=%v&%s=%v", "uid", metaQueryParam["uid"], "type", metaQueryParam["type"])

	queryUIDMetaEndpoint := fmt.Sprintf("%s%s?%s", c.tsdbEndpoint, UIDMetaDataPath, queryParam)

	uidMetaDataResp := UIDMetaDataResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err := c.sendRequest(GetMethod, queryUIDMetaEndpoint, "", &uidMetaDataResp); err != nil {
		message = fmt.Sprintf("error processing query-uid-metadata request to url %q: %v", queryUIDMetaEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("query-uid-metadata request to url %q processed successfully", queryUIDMetaEndpoint)

	return &uidMetaDataResp, nil
}

func (c *Client) UpdateUIDMetaData(uidMetaData *UIDMetaData) (*UIDMetaDataResponse, error) {
	return c.operateUIDMetaData(uidMetaData, PostMethod, "UpdateUIDMetaData")
}

func (c *Client) DeleteUIDMetaData(uidMetaData *UIDMetaData) (*UIDMetaDataResponse, error) {
	return c.operateUIDMetaData(uidMetaData, DeleteMethod, "DeleteUIDMetaData")
}

func (c *Client) operateUIDMetaData(uidMetaData *UIDMetaData, method, operation string) (*UIDMetaDataResponse, error) {
	span := c.addTrace(c.ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), operation, &status, &message, span)

	if !c.isValidOperateMethod(method) {
		message = "given method for uid metadata is invalid"
		return nil, errors.New(message)
	}

	uidMetaEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, UIDMetaDataPath)

	resultBytes, err := json.Marshal(uidMetaData)
	if err != nil {
		message = fmt.Sprintf("failed to marshal uidMetaData: %v", err)
		return nil, errors.New(message)
	}

	uidMetaDataResp := UIDMetaDataResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}
	if err = c.sendRequest(method, uidMetaEndpoint, string(resultBytes), &uidMetaDataResp); err != nil {
		message = fmt.Sprintf("error processing %v request to url %q: %v", method, uidMetaEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("%v uid-metadata request to url %q processed successfully", method, uidMetaEndpoint)

	return &uidMetaDataResp, nil
}

func (uidMetaDataResp *UIDMetaDataResponse) SetStatus(code int) {
	setStatus(uidMetaDataResp.ctx, uidMetaDataResp, code, "SetStatus-UIDMetaData", uidMetaDataResp.logger)
}

func (uidMetaDataResp *UIDMetaDataResponse) setStatusCode(code int) {
	uidMetaDataResp.StatusCode = code
}

func (uidMetaDataResp *UIDMetaDataResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(uidMetaDataResp.ctx, uidMetaDataResp, "GetCustomParser-UIDMetaData", uidMetaDataResp.logger,
		func(resp []byte) error {
			if uidMetaDataResp.StatusCode == http.StatusNoContent || // The OpenTSDB deletes a UIDMetaData successfully, or
				uidMetaDataResp.StatusCode == http.StatusNotModified { // no changes were present, and with no body content.
				return nil
			}

			return json.Unmarshal(resp, &uidMetaDataResp)
		})
}

func (uidMetaDataResp *UIDMetaDataResponse) String() string {
	return toString(uidMetaDataResp.ctx, uidMetaDataResp, "ToString-UIDMetaData", uidMetaDataResp.logger)
}

// UIDAssignParam is the structure used to hold
// the parameters when calling POST /api/uid/assign.
// Each attributes in UIDAssignParam matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/uid/assign.html.
type UIDAssignParam struct {
	// An optional list of metric names for assignment
	Metric []string `json:"metric,omitempty"`

	// An optional list of tag names for assignment
	Tagk []string `json:"tagk,omitempty"`

	// An optional list of tag values for assignment
	Tagv []string `json:"tagv,omitempty"`
}

// UIDAssignResponse acts as the implementation of Response in the POST /api/uid/assign scene.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/uid/assign.html.
type UIDAssignResponse struct {
	StatusCode   int
	Metric       map[string]string `json:"metric"`
	MetricErrors map[string]string `json:"metric_errors,omitempty"`
	Tagk         map[string]string `json:"tagk"`
	TagkErrors   map[string]string `json:"tagk_errors,omitempty"`
	Tagv         map[string]string `json:"tagv"`
	TagvErrors   map[string]string `json:"tagv_errors,omitempty"`
	logger       Logger
	tracer       trace.Tracer
	ctx          context.Context
}

func (c *Client) AssignUID(assignParam *UIDAssignParam) (*UIDAssignResponse, error) {
	span := c.addTrace(c.ctx, "AssignUID")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "AssignUID", &status, &message, span)

	assignUIDEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, UIDAssignPath)

	resultBytes, err := json.Marshal(assignParam)
	if err != nil {
		message = fmt.Sprintf("failed to marshal UIDAssignParam: %v", err)
		return nil, errors.New(message)
	}

	uidAssignResp := UIDAssignResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err = c.sendRequest(PostMethod, assignUIDEndpoint, string(resultBytes), &uidAssignResp); err != nil {
		message = fmt.Sprintf("error processing %v request to url %q: %v", PostMethod, assignUIDEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = "assign UID successful"

	return &uidAssignResp, nil
}

func (uidAssignResp *UIDAssignResponse) SetStatus(code int) {
	setStatus(uidAssignResp.ctx, uidAssignResp, code, "SetStatus-UIDAssign", uidAssignResp.logger)
}

func (uidAssignResp *UIDAssignResponse) setStatusCode(code int) {
	uidAssignResp.StatusCode = code
}

func (*UIDAssignResponse) GetCustomParser() func(respCnt []byte) error {
	return nil
}

func (uidAssignResp *UIDAssignResponse) String() string {
	return toString(uidAssignResp.ctx, uidAssignResp, "ToString-UIDAssign", uidAssignResp.logger)
}

// TSMetaData is the structure used to hold
// the parameters when calling (POST,PUT,DELETE) /api/uid/tsmeta.
// Each attributes in TSMetaData matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/uid/tsmeta.html.
type TSMetaData struct {
	// A required hexadecimal representation of the timeseries UID
	TsUID string `json:"tsuid,omitempty"`

	// An optional brief description of what the UID represents
	Description string `json:"description,omitempty"`

	// An optional short name that can be displayed in GUIs instead of the default name
	DisplayName string `json:"displayName,omitempty"`

	// An optional detailed notes about what the UID represents
	Notes string `json:"notes,omitempty"`

	// An optional key/value map to store custom fields and values
	Custom map[string]string `json:"custom,omitempty"`

	// An optional value reflective of the data stored in the timeseries, may be used in GUIs or calculations
	Units string `json:"units,omitempty"`

	// The kind of data stored in the timeseries such as counter, gauge, absolute, etc.
	// These may be defined later, but they should be similar to Data Source Types in an RRD.
	// Its value is optional
	DataType string `json:"dataType,omitempty"`

	// The number of days of data points to retain for the given timeseries. Not Implemented.
	// When set to 0, the default, data is retained indefinitely.
	// Its value is optional
	Retention int64 `json:"retention,omitempty"`

	// An optional maximum value for this timeseries that may be used in calculations such as
	// percent of maximum. If the default of NaN is present, the value is ignored.
	Max float64 `json:"max,omitempty"`

	// An optional minimum value for this timeseries that may be used in calculations such as
	// percent of minimum. If the default of NaN is present, the value is ignored.
	Min float64 `json:"min,omitempty"`
}

type TSMetaDataResponse struct {
	StatusCode int
	TSMetaData
	Metric          UIDMetaData            `json:"metric,omitempty"`
	Tags            []UIDMetaData          `json:"tags,omitempty"`
	Created         int64                  `json:"created,omitempty"`
	LastReceived    int64                  `json:"lastReceived,omitempty"`
	TotalDatapoints int64                  `json:"totalDatapoints,omitempty"`
	ErrorInfo       map[string]interface{} `json:"error,omitempty"`
	logger          Logger
	tracer          trace.Tracer
	ctx             context.Context
}

func (c *Client) QueryTSMetaData(tsuid string) (*TSMetaDataResponse, error) {
	span := c.addTrace(c.ctx, "QueryTSMetaData")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryTSMetaData", &status, &message, span)

	tsuid = strings.TrimSpace(tsuid)

	if tsuid == "" {
		message = "tsuid is empty"
		return nil, errors.New(message)
	}

	queryTSMetaEndpoint := fmt.Sprintf("%s%s?tsuid=%s", c.tsdbEndpoint, TSMetaDataPath, tsuid)
	tsMetaDataResp := TSMetaDataResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err := c.sendRequest(http.MethodGet, queryTSMetaEndpoint, "", &tsMetaDataResp); err != nil {
		message = fmt.Sprintf("error processing %v request to url %q: %v", http.MethodGet, queryTSMetaEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = "query TSMetaData successful"

	return &tsMetaDataResp, nil
}

func (c *Client) UpdateTSMetaData(tsMetaData *TSMetaData) (*TSMetaDataResponse, error) {
	return c.operateTSMetaData(tsMetaData, PostMethod, "UpdateTSMetaData")
}

func (c *Client) DeleteTSMetaData(tsMetaData *TSMetaData) (*TSMetaDataResponse, error) {
	return c.operateTSMetaData(tsMetaData, DeleteMethod, "DeleteTSMetaData")
}

func (c *Client) operateTSMetaData(tsMetaData *TSMetaData, method, operation string) (*TSMetaDataResponse, error) {
	span := c.addTrace(c.ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), operation, &status, &message, span)

	if !c.isValidOperateMethod(method) {
		message = fmt.Sprintf("The %s method for operating a uid metadata is invalid", method)
		return nil, errors.New(message)
	}

	tsMetaEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, TSMetaDataPath)

	resultBytes, err := json.Marshal(tsMetaData)
	if err != nil {
		message = fmt.Sprintf("failed to marshal %s response: %v", operation, err)
		return nil, errors.New(message)
	}

	tsMetaDataResp := TSMetaDataResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err = c.sendRequest(method, tsMetaEndpoint, string(resultBytes), &tsMetaDataResp); err != nil {
		message = fmt.Sprintf("failed to send request at url %q: %v", tsMetaEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = "operateTSMetaData request processed successfully"

	return &tsMetaDataResp, nil
}

func (tsMetaDataResp *TSMetaDataResponse) SetStatus(code int) {
	setStatus(tsMetaDataResp.ctx, tsMetaDataResp, code, "SetStatus-TSMetaData", tsMetaDataResp.logger)
}

func (tsMetaDataResp *TSMetaDataResponse) setStatusCode(code int) {
	tsMetaDataResp.StatusCode = code
}

func (tsMetaDataResp *TSMetaDataResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(tsMetaDataResp.ctx, tsMetaDataResp, "GetCustomParser-TSMetaData", tsMetaDataResp.logger,
		func(resp []byte) error {
			if tsMetaDataResp.StatusCode == http.StatusNoContent ||
				tsMetaDataResp.StatusCode == http.StatusNotModified {
				return nil
			}

			return json.Unmarshal(resp, &tsMetaDataResp)
		})
}

func (tsMetaDataResp *TSMetaDataResponse) String() string {
	return toString(tsMetaDataResp.ctx, tsMetaDataResp, "ToString-TSMetaData", tsMetaDataResp.logger)
}
