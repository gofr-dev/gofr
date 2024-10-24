package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// SuggestParam is the structure used to hold
// the querying parameters when calling /api/suggest.
// Each attributes in SuggestParam matches the definition in
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/suggest.html.
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
	ctx    context.Context
}

func (sugParam *SuggestParam) String() string {
	return toString(sugParam.ctx, sugParam, "ToString-SuggestParam", sugParam.logger)
}

func (*SuggestParam) setStatusCode(int) {
	// not implemented
}

type SuggestResponse struct {
	StatusCode int
	ResultInfo []string `json:"ResultInfo"`
	logger     Logger
	tracer     trace.Tracer
	ctx        context.Context
}

func (sugResp *SuggestResponse) SetStatus(code int) {
	setStatus(sugResp.ctx, sugResp, code, "SetStatus-Suggest", sugResp.logger)
}

func (sugResp *SuggestResponse) setStatusCode(code int) {
	sugResp.StatusCode = code
}

func (sugResp *SuggestResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(sugResp.ctx, sugResp, "GetCustomParser-Suggest", sugResp.logger,
		func(resp []byte) error {
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"ResultInfo"`, string(resp))), &sugResp)
		})
}

func (sugResp *SuggestResponse) String() string {
	return toString(sugResp.ctx, sugResp, "ToString-VersionResp", sugResp.logger)
}

func (c *Client) Suggest(sugParam *SuggestParam) (*SuggestResponse, error) {
	if sugParam.logger == nil {
		sugParam.logger = c.logger
	}

	if sugParam.tracer == nil {
		sugParam.tracer = c.tracer
	}

	span := c.addTrace(context.Background(), "Suggest")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Suggest", &status, &message, span)

	if !isValidSuggestParam(sugParam) {
		message = "invalid suggest param"
		return nil, errors.New(message)
	}

	sugEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, SuggestPath)

	reqBodyCnt, err := getSuggestBodyContents(sugParam)
	if err != nil {
		message = fmt.Sprintf("get suggest body content error: %s", err)
		return nil, err
	}

	sugResp := SuggestResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}
	if err := c.sendRequest(PostMethod, sugEndpoint, reqBodyCnt, &sugResp); err != nil {
		message = fmt.Sprintf("error processing suggest request to url %q: %s", sugEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("suggest query request to url %q processed successfully", sugEndpoint)

	return &sugResp, nil
}

func isValidSuggestParam(sugParam *SuggestParam) bool {
	if sugParam.Type == "" {
		return false
	}

	sugParam.Type = strings.TrimSpace(sugParam.Type)

	types := []string{TypeMetrics, TypeTagk, TypeTagv}
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
		return "", fmt.Errorf("failed to marshal suggest param: %v", err)
	}

	return string(result), nil
}
