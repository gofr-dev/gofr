package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// VersionResponse is the struct implementation for /api/version.
type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]any
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

func (verResp *VersionResponse) SetStatus(code int) {
	setStatus(verResp.ctx, verResp, code, "SetStatus-VersionResp", verResp.logger)
}

func (verResp *VersionResponse) setStatusCode(code int) {
	verResp.StatusCode = code
}

func (verResp *VersionResponse) String() string {
	return toString(verResp.ctx, verResp, "ToString-VersionResp", verResp.logger)
}

func (verResp *VersionResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(verResp.ctx, verResp, "GetCustomParser-VersionResp", verResp.logger,
		func(resp []byte) error {
			v := make(map[string]any, 0)

			err := json.Unmarshal(resp, &v)
			if err != nil {
				return err
			}

			verResp.VersionInfo = v

			return nil
		})
}

func (c *Client) version(ctx context.Context) (*VersionResponse, error) {
	span := c.addTrace(ctx, "Version")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.endpoint, VersionPath)
	verResp := VersionResponse{logger: c.logger, tracer: c.tracer, ctx: ctx}

	if err := c.sendRequest(ctx, http.MethodGet, verEndpoint, "", &verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = "version response retrieved successfully."

	return &verResp, nil
}
