package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type GenericResponse interface {
	addTrace(ctx context.Context, operation string) (context.Context, trace.Span)
	setStatusCode(code int)
}

func toString(resp GenericResponse, ctx context.Context, operation string, logger Logger) string {
	_, span := resp.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(resp)
	if err != nil {
		message = fmt.Sprintf("%s marshal response error: %s", operation, err)
		logger.Errorf(message)

		return ""
	}

	fmt.Fprintf(buffer, "%s\n", string(content))

	status = StatusSuccess
	message = fmt.Sprintf("%s response converted to string successfully", operation)

	return buffer.String()
}

func setStatus(resp GenericResponse, ctx context.Context, code int, operation string, logger Logger) {
	_, span := resp.addTrace(ctx, operation)

	status := StatusSuccess
	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)
	message = fmt.Sprintf("Set response code to: %d", code)

	resp.setStatusCode(code)
}

func getCustomParser(resp GenericResponse, ctx context.Context, operation string, logger Logger,
	unmarshalFunc func([]byte, interface{}) error) func([]byte) error {
	_, span := resp.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)

	return func(result []byte) error {
		err := unmarshalFunc(result, &resp)
		if err != nil {
			message = fmt.Sprintf("unmarshal %s error: %s", operation, err)
			logger.Errorf(message)

			return err
		}

		status = StatusSuccess
		message = fmt.Sprintf("%s custom parsing was successful.", operation)
		return nil
	}
}
