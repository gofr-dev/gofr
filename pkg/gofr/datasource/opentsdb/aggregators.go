package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
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
	setStatus(aggreResp, ctx, code, "SetStatus-Aggregator", aggreResp.logger)
}

func (aggreResp *AggregatorsResponse) setStatusCode(code int) {
	aggreResp.StatusCode = code
}

func (aggreResp *AggregatorsResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	return getCustomParser(aggreResp, ctx, "GetCustomParser-Aggregator", aggreResp.logger,
		func(resp []byte, target interface{}) error {
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &aggreResp)
		})
}

func (aggreResp *AggregatorsResponse) String(ctx context.Context) string {
	return toString(aggreResp, ctx, "ToString-Aggregators", aggreResp.logger)
}

func (c *OpentsdbClient) Aggregators() (*AggregatorsResponse, error) {
	_, span := c.addTrace(c.ctx, "Aggregators")
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
