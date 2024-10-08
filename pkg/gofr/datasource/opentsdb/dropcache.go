package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type DropcachesResponse struct {
	StatusCode     int
	DropcachesInfo map[string]string `json:"DropcachesInfo"`
	logger         Logger
	tracer         trace.Tracer
}

func (dropResp *DropcachesResponse) SetStatus(ctx context.Context, code int) {
	_, span := dropResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(dropResp.logger, time.Now(), "SetStatus-DropCacheResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	dropResp.StatusCode = code
}

func (dropResp *DropcachesResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := dropResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(dropResp.logger, time.Now(), "GetCustomParser-DropCacheResp", &status, &message, span)

	return func(resp []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &dropResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal dropcache response error: %s", err)
			dropResp.logger.Errorf(message)

			return err
		}

		status = "SUCCESS"
		message = fmt.Sprint("Custom parsing successful")

		return nil
	}
}

func (dropResp *DropcachesResponse) String(ctx context.Context) string {
	_, span := dropResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(dropResp.logger, time.Now(), "ToString-ConfigResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(dropResp)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		dropResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("config response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Dropcaches() (*DropcachesResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "Dropcaches")
	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Dropcaches", &status, &message, span)
	dropEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, DropcachesPath)
	dropResp := DropcachesResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, dropEndpoint, "", &dropResp); err != nil {
		message = fmt.Sprintf("error processing drop cache request at url %s: %s", dropEndpoint, err)
		return nil, err
	}
	status = "SUCCESS"
	message = fmt.Sprintf("drop cache processed successfully at url: %s", dropEndpoint)

	return &dropResp, nil
}
