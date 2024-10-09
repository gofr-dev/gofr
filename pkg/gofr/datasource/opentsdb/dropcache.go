package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type DropcachesResponse struct {
	StatusCode     int
	DropcachesInfo map[string]string `json:"DropcachesInfo"`
	logger         Logger
	tracer         trace.Tracer
	ctx            context.Context
}

func (dropResp *DropcachesResponse) SetStatus(code int) {
	setStatus(dropResp.ctx, dropResp, code, "SetStatus-Dropcaches", dropResp.logger)
}

func (dropResp *DropcachesResponse) setStatusCode(code int) {
	dropResp.StatusCode = code
}

func (dropResp *DropcachesResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(dropResp.ctx, dropResp, "GetCustomParser-Dropcaches", dropResp.logger,
		func(resp []byte) error {
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"DropcachesInfo"`, string(resp))), &dropResp)
		})
}

func (dropResp *DropcachesResponse) String() string {
	return toString(dropResp.ctx, dropResp, "ToString-Dropcache", dropResp.logger)
}

func (c *OpentsdbClient) Dropcaches() (*DropcachesResponse, error) {
	span := c.addTrace(c.ctx, "Dropcaches")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Dropcaches", &status, &message, span)

	dropEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, DropcachesPath)
	dropResp := DropcachesResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err := c.sendRequest(GetMethod, dropEndpoint, "", &dropResp); err != nil {
		message = fmt.Sprintf("error processing drop cache request at url %q: %s", dropEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("drop cache processed successfully at url %q", dropEndpoint)

	return &dropResp, nil
}
