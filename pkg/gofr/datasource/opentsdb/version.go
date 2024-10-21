package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// VersionResponse is the struct implementation for /api/version.
type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]string `json:"VersionInfo"`
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
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"VersionInfo"`, string(resp))), &verResp)
		})
}

func (c *OpentsdbClient) version() (*VersionResponse, error) {
	span := c.addTrace(c.ctx, "Version")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, VersionPath)
	verResp := VersionResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err := c.sendRequest(GetMethod, verEndpoint, "", &verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = "version response retrieved successfully."

	return &verResp, nil
}
