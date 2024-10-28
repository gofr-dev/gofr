package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/aggregators.html.
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
			j := make([]string, 0)

			err := json.Unmarshal(resp, &j)
			if err != nil {
				return err
			}

			aggreResp.Aggregators = j

			return nil
		})
}

func (aggreResp *AggregatorsResponse) String() string {
	return toString(aggreResp.ctx, aggreResp, "ToString-Aggregators", aggreResp.logger)
}

func (c *Client) Aggregators(ctx context.Context) (*AggregatorsResponse, error) {
	span := c.addTrace(ctx, "Aggregators")
	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Aggregators", &status, &message, span)

	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.endpoint, AggregatorPath)

	aggreResp := AggregatorsResponse{logger: c.logger, tracer: c.tracer, ctx: ctx}

	if err := c.sendRequest(ctx, http.MethodGet, aggregatorsEndpoint, "", &aggreResp); err != nil {
		message = fmt.Sprintf("error retrieving aggregators from url: %s", aggregatorsEndpoint)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("aggregators retrieved from url: %s", aggregatorsEndpoint)

	c.logger.Log("aggregators fetched successfully")

	return &aggreResp, nil
}
