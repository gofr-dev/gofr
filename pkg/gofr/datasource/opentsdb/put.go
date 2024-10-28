package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

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

func (data *DataPoint) String() string {
	content, _ := json.Marshal(data)
	return string(content)
}

// PutError holds the error message for each putting DataPoint instance. Only calling PUT() with "details"
// query parameter, the response of the failed put data operation can contain an array PutError instance
// to show the details for each failure.
type PutError struct {
	Data     DataPoint `json:"datapoint"`
	ErrorMsg string    `json:"error"`
}

func (putErr *PutError) String() string {
	return fmt.Sprintf("%s:%s", putErr.ErrorMsg, putErr.Data.String())
}

// PutResponse acts as the implementation of Response in the /api/put scene.
// It holds the status code and the response values defined in
// the [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/put.html.
type PutResponse struct {
	StatusCode int
	Failed     int64      `json:"failed"`
	Success    int64      `json:"success"`
	Errors     []PutError `json:"errors,omitempty"`
	logger     Logger
	tracer     trace.Tracer
	ctx        context.Context
}

func (putResp *PutResponse) SetStatus(code int) {
	setStatus(putResp.ctx, putResp, code, "SetStatus-PutResponse", putResp.logger)
}

func (putResp *PutResponse) setStatusCode(code int) {
	putResp.StatusCode = code
}

func (putResp *PutResponse) String() string {
	return toString(putResp.ctx, putResp, "ToString-PutResponse", putResp.logger)
}

func (*PutResponse) GetCustomParser() func(respCnt []byte) error {
	return nil
}

func (c *Client) Put(ctx context.Context, datas []DataPoint, queryParam string) (*PutResponse, error) {
	span := c.addTrace(ctx, "Put")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Put", &status, &message, span)

	err := validateDataPoint(datas)
	if err != nil {
		message = fmt.Sprintf("invalid data: %s", err)
		return nil, err
	}

	if !isValidPutParam(queryParam) {
		message = "The given query param is invalid."
		return nil, errors.New(message)
	}

	var putEndpoint string
	if !isEmptyPutParam(queryParam) {
		putEndpoint = fmt.Sprintf("%s%s?%s", c.endpoint, PutPath, queryParam)
	} else {
		putEndpoint = fmt.Sprintf("%s%s", c.endpoint, PutPath)
	}

	dataGroups, err := c.splitProperGroups(ctx, datas)
	if err != nil {
		message = fmt.Sprintf("split data point error: %s", err)
		return nil, err
	}

	responses := make([]PutResponse, 0)

	responses, err = c.getResponses(ctx, putEndpoint, dataGroups, responses, &message)
	if err != nil {
		return nil, err
	}

	globalResp := PutResponse{logger: c.logger, tracer: c.tracer, ctx: ctx}
	globalResp.StatusCode = http.StatusOK

	for _, resp := range responses {
		globalResp.Failed += resp.Failed
		globalResp.Success += resp.Success
		globalResp.Errors = append(globalResp.Errors, resp.Errors...)

		if resp.StatusCode != http.StatusOK && globalResp.StatusCode == http.StatusOK {
			globalResp.StatusCode = resp.StatusCode
		}
	}

	if globalResp.StatusCode == http.StatusOK {
		status = StatusSuccess
		message = fmt.Sprintf("Put request to url %q processed successfully", putEndpoint)

		return &globalResp, nil
	}

	return nil, parsePutErrorMsg(&globalResp)
}

func (c *Client) getResponses(ctx context.Context, putEndpoint string, dataGroups [][]DataPoint,
	responses []PutResponse, message *string) ([]PutResponse, error) {
	for _, datapoints := range dataGroups {
		reqBodyCnt, err := getPutBodyContents(datapoints)
		if err != nil {
			*message = fmt.Sprintf("getPutBodyContents error: %s", err)
			c.logger.Errorf(*message)
		}

		putResp := PutResponse{logger: c.logger, tracer: c.tracer, ctx: ctx}

		if err = c.sendRequest(ctx, http.MethodPost, putEndpoint, reqBodyCnt, &putResp); err != nil {
			*message = fmt.Sprintf("error processing put request at url %q: %s", putEndpoint, err)
			return nil, err
		}

		responses = append(responses, putResp)
	}

	return responses, nil
}

// splitProperGroups splits the given datapoints into groups, whose content size is not larger than
// c.opentsdbCfg.MaxContentLength. This method is to avoid Put failure, when the content length of
// the given datapoints in a single /api/put request exceeded the value of
// tsd.http.request.max_chunk in the opentsdb config file.
func (c *Client) splitProperGroups(ctx context.Context, datapoints []DataPoint) ([][]DataPoint, error) {
	span := c.addTrace(ctx, "splitProperGroups-Put")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "splitProperGroups-Put", &status, &message, span)

	datasBytes, err := json.Marshal(&datapoints)
	if err != nil {
		message = fmt.Sprintf("failed to marshal the datapoints to be put: %v", err)
		return nil, errors.New(message)
	}

	datapointGroups := make([][]DataPoint, 0)
	datapointGroups = c.appendDataPoints(datasBytes, datapoints, datapointGroups)
	status = StatusSuccess
	message = "splitting into groups successful"

	return datapointGroups, nil
}

func (c *Client) appendDataPoints(datasBytes []byte, datapoints []DataPoint, datapointGroups [][]DataPoint) [][]DataPoint {
	if len(datasBytes) > c.config.MaxContentLength {
		return c.splitLargeDataPoints(datapoints, datapointGroups)
	}

	return append(datapointGroups, datapoints)
}

func (c *Client) splitLargeDataPoints(datapoints []DataPoint, datapointGroups [][]DataPoint) [][]DataPoint {
	datapointsSize := len(datapoints)
	startIndex := 0
	endIndex := c.calculateEndIndex(datapointsSize)

	for endIndex <= datapointsSize {
		tempdps := datapoints[startIndex:endIndex]
		if c.canAppendGroup(tempdps) {
			datapointGroups = append(datapointGroups, tempdps)
			startIndex = endIndex
			endIndex = c.calculateNextEndIndex(startIndex, datapointsSize, len(tempdps))
		} else {
			endIndex -= c.config.DetectDeltaNum
		}

		if startIndex >= datapointsSize {
			break
		}
	}

	return datapointGroups
}

func (c *Client) calculateEndIndex(datapointsSize int) int {
	if datapointsSize > c.config.MaxPutPointsNum {
		return c.config.MaxPutPointsNum
	}

	return datapointsSize
}

func (*Client) calculateNextEndIndex(startIndex, datapointsSize, tempSize int) int {
	endIndex := startIndex + tempSize
	if endIndex > datapointsSize {
		return datapointsSize
	}

	return endIndex
}

func (c *Client) canAppendGroup(datapoints []DataPoint) bool {
	tempdpsBytes, _ := json.Marshal(&datapoints)
	return len(tempdpsBytes) <= c.config.MaxContentLength
}

func parsePutErrorMsg(resp *PutResponse) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Failed to put %d datapoint(s) into opentsdb, statuscode %d:\n", resp.Failed, resp.StatusCode))

	if len(resp.Errors) > 0 {
		for _, putError := range resp.Errors {
			buf.WriteString(fmt.Sprintf("\t%s\n", putError.String()))
		}
	}

	return errors.New(buf.String())
}

func getPutBodyContents(datas []DataPoint) (string, error) {
	if len(datas) == 1 {
		result, err := json.Marshal(datas[0])
		if err != nil {
			return "", fmt.Errorf("failed to marshal datapoint: %v", err)
		}

		return string(result), nil
	}

	reqBodyCnt, err := marshalDataPoints(datas)
	if err != nil {
		return "", fmt.Errorf("failed to marshal datapoint: %v", err)
	}

	return reqBodyCnt, nil
}

func marshalDataPoints(datas []DataPoint) (string, error) {
	buffer := bytes.NewBuffer(nil)

	result, err := json.Marshal(datas)
	if err != nil {
		return "", err
	}

	buffer.Write(result)

	return buffer.String(), nil
}

func validateDataPoint(datas []DataPoint) error {
	if len(datas) == 0 {
		return errors.New("the given datapoint is empty")
	}

	for _, data := range datas {
		if !isValidDataPoint(&data) {
			return errors.New("the value of the given datapoint is invalid")
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
		return true
	case int:
		return true
	case float64:
		return true
	case float32:
		return true
	case string:
		return true
	default:
		return false
	}
}

func isValidPutParam(param string) bool {
	if isEmptyPutParam(param) {
		return true
	}

	param = strings.TrimSpace(param)
	if param != PutRespWithSummary && param != PutRespWithDetails {
		return false
	}

	return true
}

func isEmptyPutParam(param string) bool {
	return strings.TrimSpace(param) == ""
}
