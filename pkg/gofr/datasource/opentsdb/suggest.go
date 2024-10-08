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

// SuggestParam is the structure used to hold
// the querying parameters when calling /api/suggest.
// Each attributes in SuggestParam matches the definition in
// (http://opentsdb.net/docs/build/html/api_http/suggest.html).
type SuggestParam struct {
	// The type of data to auto complete on.
	// Must be one of the following: metrics, tagk or tagv.
	// It is required.
	// Only the one of the three query type can be used:
	// TypeMetrics, TypeTagk, TypeTagv
	Type string `json:"type"`

	// An optional string value to match on for the given type
	Q string `json:"q,omitempty"`

	// An optional integer value presenting the maximum number of suggested
	// results to return. If it is set, it must be greater than 0.
	MaxResultNum int `json:"max,omitempty"`

	logger Logger
	tracer trace.Tracer
}

func (sugParam *SuggestParam) String(ctx context.Context) string {
	_, span := sugParam.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(sugParam.logger, time.Now(), "ToString-SuggestResp", &status, &message, span)

	content, err := json.Marshal(sugParam)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		sugParam.logger.Errorf(message)
	}

	status = "SUCCESS"
	message = fmt.Sprint("suggest response converted to string successfully")

	return string(content)
}

type SuggestResponse struct {
	StatusCode int
	ResultInfo []string `json:"ResultInfo"`
	logger     Logger
	tracer     trace.Tracer
}

func (sugResp *SuggestResponse) SetStatus(ctx context.Context, code int) {
	_, span := sugResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(sugResp.logger, time.Now(), "SetStatus-suggestResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	sugResp.StatusCode = code
}

func (sugResp *SuggestResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := sugResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(sugResp.logger, time.Now(), "GetCustomParser-SuggestResp", &status, &message, span)

	return func(respCnt []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"ResultInfo"`, string(respCnt))), &sugResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal suggest response error: %s", err)
			sugResp.logger.Errorf(message)
		}

		status = "SUCCESS"
		message = fmt.Sprintf("custom parsing successful")

		return nil
	}
}

func (sugResp *SuggestResponse) String(ctx context.Context) string {
	_, span := sugResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(sugResp.logger, time.Now(), "ToString-SuggestResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(sugResp)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		sugResp.logger.Errorf(message)
	}

	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("suggest response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Suggest(sugParam SuggestParam) (*SuggestResponse, error) {
	if sugParam.logger == nil {
		sugParam.logger = c.logger
	}

	if sugParam.tracer == nil {
		sugParam.tracer = c.tracer
	}

	tracedCtx, span := c.addTrace(context.Background(), "Suggest")
	c.ctx = tracedCtx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Suggest", &status, &message, span)

	if !isValidSuggestParam(&sugParam) {
		message = "invalid suggest param"
		return nil, errors.New(message)
	}

	sugEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, SuggestPath)
	reqBodyCnt, err := getSuggestBodyContents(&sugParam)
	if err != nil {
		message = fmt.Sprintf("get suggest body content error: %s", err)
		return nil, err
	}

	sugResp := SuggestResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(PostMethod, sugEndpoint, reqBodyCnt, &sugResp); err != nil {
		message = fmt.Sprintf("error processing suggest request to url %s: %s", sugEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("")
	return &sugResp, nil
}

func isValidSuggestParam(sugParam *SuggestParam) bool {
	if sugParam.Type == "" {
		return false
	}
	types := []string{TypeMetrics, TypeTagk, TypeTagv}
	sugParam.Type = strings.TrimSpace(sugParam.Type)
	for _, typeItem := range types {
		if sugParam.Type == typeItem {
			return true
		}
	}
	return false
}

func getSuggestBodyContents(sugParam *SuggestParam) (string, error) {
	result, err := json.Marshal(sugParam)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to marshal suggest param: %v\n", err))
	}
	return string(result), nil
}
