package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Annotation holds parameters for querying or managing annotations via the /api/annotation endpoint in OpenTSDB.
// Used for logging notes on events at specific times, often tied to time series data, mainly for graphing or API queries.
type Annotation struct {
	// StartTime is the Unix epoch timestamp (in seconds) for when the event occurred. This is required.
	StartTime int64 `json:"startTime,omitempty"`

	// EndTime is the optional Unix epoch timestamp (in seconds) for when the event ended, if applicable.
	EndTime int64 `json:"endTime,omitempty"`

	// Tsuid is the optional time series identifier if the annotation is linked to a specific time series.
	Tsuid string `json:"tsuid,omitempty"`

	// Description is a brief, optional summary of the event (recommended to keep under 25 characters for display purposes).
	Description string `json:"description,omitempty"`

	// Notes is an optional, detailed description of the event.
	Notes string `json:"notes,omitempty"`

	// Custom is an optional key/value map to store any additional fields and their values.
	Custom map[string]string `json:"custom,omitempty"`
}

// AnnotationResponse encapsulates the response data and status when interacting with the /api/annotation endpoint.
type AnnotationResponse struct {
	// StatusCode holds the HTTP status code for the annotation request.
	StatusCode int

	// Annotation holds the associated annotation object.
	Annotation

	// ErrorInfo contains details about any errors that occurred during the request.
	ErrorInfo map[string]interface{} `json:"error,omitempty"`

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

func (annotResp *AnnotationResponse) SetStatus(code int) {
	setStatus(annotResp.ctx, annotResp, code, "SetStatus-Annotation", annotResp.logger)
}

func (annotResp *AnnotationResponse) setStatusCode(code int) {
	annotResp.StatusCode = code
}

func (annotResp *AnnotationResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(annotResp.ctx, annotResp, "GetCustomParser-Annotation", annotResp.logger,
		func(resp []byte) error {
			originContents := string(resp)

			var resultBytes []byte

			if strings.Contains(originContents, "startTime") || strings.Contains(originContents, "error") {
				resultBytes = resp
			} else if annotResp.StatusCode == http.StatusNoContent {
				return nil
			}

			return json.Unmarshal(resultBytes, &annotResp)
		})
}

func (annotResp *AnnotationResponse) String() string {
	return toString(annotResp.ctx, annotResp, "ToString-Annotation", annotResp.logger)
}

func (c *OpentsdbClient) QueryAnnotation(queryAnnoParam map[string]interface{}) (*AnnotationResponse, error) {
	span := c.addTrace(c.ctx, "QueryAnnotation")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryAnnotation", &status, &message, span)

	if len(queryAnnoParam) == 0 {
		message = "annotation query parameter is empty"
		return nil, errors.New(message)
	}

	buffer := bytes.NewBuffer(nil)

	size := len(queryAnnoParam)

	i := 0

	for k, v := range queryAnnoParam {
		fmt.Fprintf(buffer, "%s=%v", k, v)

		if i < size-1 {
			buffer.WriteString("&")
		}

		i++
	}

	annoEndpoint := fmt.Sprintf("%s%s?%s", c.tsdbEndpoint, AnnotationPath, buffer.String())
	annResp := AnnotationResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err := c.sendRequest(GetMethod, annoEndpoint, "", &annResp); err != nil {
		message = fmt.Sprintf("error while processing annotation query: %s", err.Error())
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("Annotation query sent to url: %s", annoEndpoint)

	c.logger.Logf("Annotation query processed successfully")

	return &annResp, nil
}

func (c *OpentsdbClient) UpdateAnnotation(annotation *Annotation) (*AnnotationResponse, error) {
	return c.operateAnnotation(annotation, PostMethod, "UpdateAnnoation")
}

func (c *OpentsdbClient) DeleteAnnotation(annotation *Annotation) (*AnnotationResponse, error) {
	return c.operateAnnotation(annotation, DeleteMethod, "DeleteAnnotation")
}

func (c *OpentsdbClient) operateAnnotation(annotation *Annotation, method, operation string) (*AnnotationResponse, error) {
	span := c.addTrace(c.ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), operation, &status, &message, span)

	if !c.isValidOperateMethod(method) {
		message = fmt.Sprintf("invalid annotation operation method: %s", method)
		return nil, errors.New(message)
	}

	annoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AnnotationPath)

	resultBytes, err := json.Marshal(annotation)
	if err != nil {
		message = fmt.Sprintf("marshal annotation response error: %s", err)
		return nil, errors.New(message)
	}

	annResp := AnnotationResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}

	if err = c.sendRequest(method, annoEndpoint, string(resultBytes), &annResp); err != nil {
		message = fmt.Sprintf("%s: error while processing %s annotation request to url %q: %s", operation, method, annoEndpoint, err.Error())
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("%s: %s annotation request to url %q processed successfully", operation, method, annoEndpoint)

	c.logger.Logf("%s request successful", operation)

	return &annResp, nil
}

// BulkAnnotatResponse represents the response structure for bulk annotation updates or deletes
// via the /api/annotation/bulk endpoint.
type BulkAnnotatResponse struct {
	// StatusCode holds the HTTP status code of the bulk annotation request.
	StatusCode int

	// UpdateAnnotations holds the list of annotations involved in a bulk update.
	UpdateAnnotations []Annotation `json:"InvolvedAnnotations,omitempty"`

	// ErrorInfo contains details about any errors that occurred during the bulk operation.
	ErrorInfo map[string]interface{} `json:"error,omitempty"`

	// Tsuids holds the list of TSUIDs for annotations that should be deleted.
	// If empty or nil, the global flag is used.
	Tsuids []string `json:"tsuids,omitempty"`

	// StartTime is the Unix epoch timestamp for the start of the deletion request.
	StartTime int64 `json:"startTime,omitempty"`

	// EndTime is the optional Unix epoch timestamp for when the events ended.
	EndTime int64 `json:"endTime,omitempty"`

	// Global indicates whether global annotations should be deleted for the given time range.
	Global bool `json:"global,omitempty"`

	// TotalDeleted holds the total number of annotations successfully deleted in the bulk operation.
	TotalDeleted int64 `json:"totalDeleted,omitempty"`

	logger Logger
	tracer trace.Tracer
	ctx    context.Context
}

// BulkAnnoDeleteInfo holds the parameters for a bulk annotation delete operation.
type BulkAnnoDeleteInfo struct {
	// Tsuids holds the list of TSUIDs for annotations that should be deleted.
	// If empty or nil, the global flag is used.
	Tsuids []string `json:"tsuids,omitempty"`

	// StartTime is the Unix epoch timestamp for the start of the deletion request.
	StartTime int64 `json:"startTime,omitempty"`

	// EndTime is the optional Unix epoch timestamp for when the events ended.
	EndTime int64 `json:"endTime,omitempty"`

	// Global indicates whether global annotations should be deleted for the given time range.
	Global bool `json:"global,omitempty"`
}

// BulkDeleteResp contains the results of a bulk annotation delete operation.
type BulkDeleteResp struct {
	BulkAnnoDeleteInfo
}

func (bulkAnnotResp *BulkAnnotatResponse) SetStatus(code int) {
	setStatus(bulkAnnotResp.ctx, bulkAnnotResp, code, "SetStatus-BulkAnnotation", bulkAnnotResp.logger)
}

func (bulkAnnotResp *BulkAnnotatResponse) setStatusCode(code int) {
	bulkAnnotResp.StatusCode = code
}

func (bulkAnnotResp *BulkAnnotatResponse) GetCustomParser() func(respCnt []byte) error {
	return getCustomParser(bulkAnnotResp.ctx, bulkAnnotResp, "GetCustomParser-BulkAnnotation", bulkAnnotResp.logger,
		func(resp []byte) error {
			originContents := string(resp)

			var resultBytes []byte

			if strings.Contains(originContents, "error") || strings.Contains(originContents, "totalDeleted") {
				resultBytes = resp
			} else if strings.Contains(originContents, "startTime") {
				resultBytes = []byte(fmt.Sprintf("{%s:%s}", `"InvolvedAnnotations"`, originContents))
			} else {
				return fmt.Errorf("unrecognized bulk annotation response: %s", originContents)
			}

			return json.Unmarshal(resultBytes, &bulkAnnotResp)
		})
}

func (bulkAnnotResp *BulkAnnotatResponse) String() string {
	return toString(bulkAnnotResp.ctx, bulkAnnotResp, "ToString-BulkAnnotation", bulkAnnotResp.logger)
}

func (c *OpentsdbClient) BulkUpdateAnnotations(annotations []Annotation) (*BulkAnnotatResponse, error) {
	span := c.addTrace(c.ctx, "BulkUpdateAnnotations")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "BulkUpdateAnnotations", &status, &message, span)

	if len(annotations) == 0 {
		message = "The annotations list is empty."
		return nil, errors.New(message)
	}

	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)

	reqBodyCnt, err := marshalAnnotations(annotations)
	if err != nil {
		message = fmt.Sprintf("Failed to marshal annotations: %v", err)
		return nil, errors.New(message)
	}

	bulkAnnoResp := BulkAnnotatResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}
	if err = c.sendRequest(PostMethod, bulkAnnoEndpoint, reqBodyCnt, &bulkAnnoResp); err != nil {
		message = fmt.Sprintf("error while processing update bulk annotations request to url %q: %s", bulkAnnoEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("Bulk annotation updation processed successfully at url %q", bulkAnnoEndpoint)

	c.logger.Logf("Bulk annotation updated successfully")

	return &bulkAnnoResp, nil
}

func (c *OpentsdbClient) BulkDeleteAnnotations(bulkDelParam *BulkAnnoDeleteInfo) (*BulkAnnotatResponse, error) {
	span := c.addTrace(c.ctx, "BulkUpdateAnnotation")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "BulkUpdateAnnotations", &status, &message, span)

	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)
	resultBytes, err := json.Marshal(bulkDelParam)

	if err != nil {
		message = fmt.Sprintf("failed to marshal bulk delete request parameters: %v", err)
		return nil, errors.New(message)
	}

	bulkAnnoResp := BulkAnnotatResponse{logger: c.logger, tracer: c.tracer, ctx: c.ctx}
	if err = c.sendRequest(DeleteMethod, bulkAnnoEndpoint, string(resultBytes), &bulkAnnoResp); err != nil {
		message = fmt.Sprintf("Bulk annotation delete request failed at url %q: %v", bulkAnnoEndpoint, err)
		return nil, err
	}

	status = StatusSuccess
	message = fmt.Sprintf("Bulk annotations deleted successfully at url %q", bulkAnnoEndpoint)

	c.logger.Logf("Bulk annotation deleted successfully")

	return &bulkAnnoResp, nil
}

// marshalAnnotations converts a slice of annotations into a JSON string for bulk operations.
func marshalAnnotations(annotations []Annotation) (string, error) {
	resultBytes, err := json.Marshal(annotations)
	if err != nil {
		return "", fmt.Errorf("failed to marshal annotations: %v", err)
	}

	return string(resultBytes), nil
}
