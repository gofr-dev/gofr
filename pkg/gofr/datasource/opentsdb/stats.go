package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type StatsResponse struct {
	StatusCode int
	Metrics    []MetricInfo `json:"Metrics"`
	logger     Logger
	tracer     trace.Tracer
}

type MetricInfo struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     interface{}       `json:"value"`
	Tags      map[string]string `json:"tags"`
}

func (statsResp *StatsResponse) SetStatus(ctx context.Context, code int) {
	_, span := statsResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(statsResp.logger, time.Now(), "SetStatus-StatsResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	statsResp.StatusCode = code
}

func (statsResp *StatsResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := statsResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(statsResp.logger, time.Now(), "GetCustomParser-SuggestResp", &status, &message, span)

	return func(respCnt []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"Metrics"`, string(respCnt))), &statsResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal suggest response error: %s", err)
			statsResp.logger.Errorf(message)
		}

		status = "SUCCESS"
		message = fmt.Sprintf("custom parsing successful")

		return nil
	}
}

func (statsResp *StatsResponse) String(ctx context.Context) string {
	_, span := statsResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(statsResp.logger, time.Now(), "ToString-ConfigResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(statsResp)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		statsResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("config response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Stats() (*StatsResponse, error) {
	tracedCtx, span := c.addTrace(c.ctx, "Stats")
	c.ctx = tracedCtx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Stats", &status, &message, span)

	statsEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, StatsPath)
	statsResp := StatsResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, statsEndpoint, "", &statsResp); err != nil {
		message = fmt.Sprintf("error processing request to url %s: %s", statsEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("Stats request to %s processed successfully", statsEndpoint)
	c.logger.Logf("Stats fetched successfully)")
	return &statsResp, nil
}
