package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type ErrorResponse struct {
	StatusCode int    `json:"code"`
	Message    string `json:"message"`
}

type SerialResponse struct {
	ErrorResponse `json:"error,omitempty"` // Handles the nested "error" object
	Serializers   []Serializer             `json:"Serializers"` // Serializers field remains the same
	logger        Logger
	tracer        trace.Tracer
}

type Serializer struct {
	SerializerName string   `json:"serializer"`
	Formatters     []string `json:"formatters"`
	Parsers        []string `json:"parsers"`
	Class          string   `json:"class,omitempty"`
	ResContType    string   `json:"response_content_type,omitempty"`
	ReqContType    string   `json:"request_content_type,omitempty"`
}

func (serialResp *SerialResponse) SetStatus(ctx context.Context, code int) {
	_, span := serialResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(serialResp.logger, time.Now(), "SetStatus-serialResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	serialResp.StatusCode = code
}

func (serialResp *SerialResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := serialResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(serialResp.logger, time.Now(), "GetCustomParser-SuggestResp", &status, &message, span)

	return func(respCnt []byte) error {
		err := json.Unmarshal(respCnt, &serialResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal suggest response error: %s", err)
			return fmt.Errorf(message)
		}

		status = "SUCCESS"
		message = fmt.Sprintf("custom parsing successful")

		return nil
	}
}

func (serialResp *SerialResponse) String(ctx context.Context) string {
	_, span := serialResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(serialResp.logger, time.Now(), "ToString-SerialResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(serialResp)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		serialResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("config response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Serializers() (*SerialResponse, error) {
	_, span := c.addTrace(c.ctx, "Stats")

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Stats", &status, &message, span)
	serialEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, SerializersPath)
	serialResp := SerialResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, serialEndpoint, "", &serialResp); err != nil {
		message = fmt.Sprintf("error processing Serializer request to url %q: %s", serialEndpoint, err)
		return nil, err
	}
	return &serialResp, nil
}
