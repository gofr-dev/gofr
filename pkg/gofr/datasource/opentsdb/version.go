// version.go contains the structs and methods for the implementation of /api/version.
package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]string `json:"VersionInfo"`
	logger      Logger
	tracer      trace.Tracer
}

func (verResp *VersionResponse) SetStatus(ctx context.Context, code int) {
	setStatus(verResp, ctx, code, "SetStatus-VersionResp", verResp.logger)
}

func (verResp *VersionResponse) setStatusCode(code int) {
	verResp.StatusCode = code
}

func (verResp *VersionResponse) String(ctx context.Context) string {
	return toString(verResp, ctx, "ToString-VersionResp", verResp.logger)
}

func (verResp *VersionResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	return getCustomParser(verResp, ctx, "GetCustomParser-VersionResp", verResp.logger,
		func(resp []byte, target interface{}) error {
			return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"VersionInfo"`, string(resp))), target)
		})
}

func (c *OpentsdbClient) Version() (*VersionResponse, error) {
	tracedCtx, span := c.addTrace(c.ctx, "Version")
	c.ctx = tracedCtx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, VersionPath)
	verResp := VersionResponse{logger: c.logger, tracer: c.tracer}

	if err := c.sendRequest(GetMethod, verEndpoint, "", &verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = "version response retrieved successfully."

	return &verResp, nil
}
