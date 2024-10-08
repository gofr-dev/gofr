// version.go contains the structs and methods for the implementation of /api/version.
package opentsdb

import (
	"bytes"
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
	_, span := verResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(verResp.logger, time.Now(), "SetStatus-VersionResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	verResp.StatusCode = code
}

func (verResp *VersionResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := verResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(verResp.logger, time.Now(), "GetCustomParser-VersionResp", &status, &message, span)

	return func(resp []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"VersionInfo"`, string(resp))), &verResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal verResp response error: %s", err)
			verResp.logger.Errorf(message)
			return err
		}

		status = "SUCCESS"
		message = fmt.Sprint("Custom parsing successful")
		return nil
	}
}

func (verResp *VersionResponse) String(ctx context.Context) string {
	_, span := verResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(verResp.logger, time.Now(), "ToString-VersionResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(verResp)
	if err != nil {
		message = fmt.Sprintf("marshal version response error: %s", err.Error())
		verResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("version response converted to string successfully")
	return buffer.String()
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
		message = fmt.Sprintf("error while processing request at url %s: %s", verEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprint("version response retrieved successfully")

	return &verResp, nil
}
