package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators.
// It holds the status code and the response values defined in the
// (http://opentsdb.net/docs/build/html/api_http/aggregators.html).
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string `json:"aggregators"`
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

func (aggreResp *AggregatorsResponse) SetStatus(code int) {
	setStatus(aggreResp.ctx, aggreResp, code, "SetStatus-Aggregator", aggreResp.logger)
}

func (aggreResp *AggregatorsResponse) setStatusCode(code int) {
	aggreResp.StatusCode = code
}

func (aggreResp *AggregatorsResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(aggreResp.ctx, aggreResp, "GetCustomParser-Aggregator", aggreResp.logger,
		func(resp []byte) error {
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &aggreResp)
		})
}

func (aggreResp *AggregatorsResponse) String() string {
	return toString(aggreResp.ctx, aggreResp, "ToString-Aggregators", aggreResp.logger)
}

func (c *OpentsdbClient) Aggregators() (*AggregatorsResponse, error) {
	span := c.addTrace(c.ctx, "Aggregators")
	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Aggregators", &status, &message, span)

	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AggregatorPath)

	aggreResp := AggregatorsResponse{logger: c.logger, tracer: c.tracer}

	if err := c.sendRequest(GetMethod, aggregatorsEndpoint, "", &aggreResp); err != nil {
		message = fmt.Sprintf("error retrieving aggregators from url: %s", aggregatorsEndpoint)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("aggregators retrieved from url: %s", aggregatorsEndpoint)

	c.logger.Logf("aggregators fetched successfully")

	return &aggreResp, nil
}
