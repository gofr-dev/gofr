package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type genericResponse interface {
	addTrace(ctx context.Context, operation string) trace.Span
	setStatusCode(code int)
}

func toString(ctx context.Context, resp genericResponse, operation string, logger Logger) string {
	span := resp.addTrace(ctx, operation)

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

func setStatus(ctx context.Context, resp genericResponse, code int, operation string, logger Logger) {
	span := resp.addTrace(ctx, operation)

	status := StatusSuccess

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)
	message = fmt.Sprintf("Set response code to: %d", code)

	resp.setStatusCode(code)
}

func getCustomParser(ctx context.Context, resp genericResponse, operation string, logger Logger,
	unmarshalFunc func([]byte) error) func([]byte) error {
	span := resp.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)

	return func(result []byte) error {
		err := unmarshalFunc(result)
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

func getQueryParser(ctx context.Context, statusCode int, logger Logger, obj genericResponse, methodName string) func(respCnt []byte) error {
	return getCustomParser(ctx, obj, methodName, logger, func(resp []byte) error {
		originRespStr := string(resp)

		var respStr string

		if statusCode == http.StatusOK && strings.Contains(originRespStr, "[") && strings.Contains(originRespStr, "]") {
			respStr = fmt.Sprintf("{%s:%s}", `"queryRespCnts"`, originRespStr)
		} else {
			respStr = originRespStr
		}

		return json.Unmarshal([]byte(respStr), obj)
	})
}
