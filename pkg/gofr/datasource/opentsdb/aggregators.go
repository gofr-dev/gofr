package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators scene.
// It holds the status code and the response values defined in the
// (http://opentsdb.net/docs/build/html/api_http/aggregators.html).
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string `json:"aggregators"`
	logger      Logger
	tracer      trace.Tracer
}

func (aggreResp *AggregatorsResponse) SetStatus(ctx context.Context, code int) {
	_, span := aggreResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(aggreResp.logger, time.Now(), "SetStatus-AggregatorResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	aggreResp.StatusCode = code
}

func (aggreResp *AggregatorsResponse) GetCustomParser(ctx context.Context) func(resp []byte) error {
	_, span := aggreResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(aggreResp.logger, time.Now(), "GetCustomParser-AggregatorResp", &status, &message, span)

	return func(resp []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &aggreResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal aggregators response error: %s", err.Error())
			aggreResp.logger.Errorf(message)

			return err
		}
		status = "SUCCESS"
		message = fmt.Sprint("Custom parsing successful")

		return nil
	}
}

func (aggreResp *AggregatorsResponse) String(ctx context.Context) string {
	_, span := aggreResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(aggreResp.logger, time.Now(), "GetCustomParser-AggregatorResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(aggreResp)
	if err != nil {
		message = fmt.Sprintf("marshal aggregators response error: %s", err.Error())
		aggreResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("aggregator response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Aggregators() (*AggregatorsResponse, error) {
	tracedCtx, span := c.addTrace(c.ctx, "Aggregators")
	defer span.End()

	c.ctx = tracedCtx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Aggregators", &status, &message, span)

	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AggregatorPath)
	aggreResp := AggregatorsResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, aggregatorsEndpoint, "", &aggreResp); err != nil {
		message = fmt.Sprintf("error retrieving Aggregators from url: %s", aggregatorsEndpoint)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("Aggregators retrived from url: %s", aggregatorsEndpoint)
	c.logger.Logf("Aggregators fetched successfully")

	return &aggreResp, nil
}
