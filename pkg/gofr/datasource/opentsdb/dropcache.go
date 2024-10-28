package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type DropCachesResponse struct {
	StatusCode     int
	DropCachesInfo map[string]string
	logger         Logger
	tracer         trace.Tracer
	ctx            context.Context
}

func (dropResp *DropCachesResponse) SetStatus(code int) {
	setStatus(dropResp.ctx, dropResp, code, "SetStatus-DropCaches", dropResp.logger)
}

func (dropResp *DropCachesResponse) setStatusCode(code int) {
	dropResp.StatusCode = code
}

func (dropResp *DropCachesResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(dropResp.ctx, dropResp, "GetCustomParser-DropCaches", dropResp.logger,
		func(resp []byte) error {
			var j map[string]string

			err := json.Unmarshal(resp, &j)
			if err != nil {
				return err
			}

			dropResp.DropCachesInfo = j

			return nil
		})
}

func (dropResp *DropCachesResponse) String() string {
	return toString(dropResp.ctx, dropResp, "ToString-DropCache", dropResp.logger)
}

func (c *Client) Dropcaches(ctx context.Context) (*DropCachesResponse, error) {
	span := c.addTrace(ctx, "DropCaches")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "DropCaches", &status, &message, span)

	dropEndpoint := fmt.Sprintf("%s%s", c.endpoint, DropcachesPath)
	dropResp := DropCachesResponse{logger: c.logger, tracer: c.tracer, ctx: ctx}

	if err := c.sendRequest(ctx, http.MethodGet, dropEndpoint, "", &dropResp); err != nil {
		message = fmt.Sprintf("error processing drop cache request at url %q: %s", dropEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("drop cache processed successfully at url %q", dropEndpoint)

	return &dropResp, nil
}
