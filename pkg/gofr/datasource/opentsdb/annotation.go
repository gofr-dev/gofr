package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"strings"
	"time"
)

// Annotation represents the parameters used when querying or managing annotations
// via the /api/annotation endpoint in OpenTSDB. Annotations are simple objects
// designed to log notes about events at specific points in time, optionally associated
// with time series data. They are typically used for graphing purposes or API queries,
// not for event tracking or monitoring systems.
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
}

// SetStatus sets the HTTP status code in the AnnotationResponse.
func (annotResp *AnnotationResponse) SetStatus(ctx context.Context, code int) {
	_, span := annotResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(annotResp.logger, time.Now(), "SetStatus-AnnotationResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	annotResp.StatusCode = code
}

// GetCustomParser returns a custom parser function to process the response content
// from an annotation-related API request.
func (annotResp *AnnotationResponse) GetCustomParser(ctx context.Context) func(resp []byte) error {
	_, span := annotResp.addTrace(ctx, "GetCustomParser")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(annotResp.logger, time.Now(), "GetCustomParser-AggregatorResp", &status, &message, span)

	return func(resp []byte) error {
		originContents := string(resp)

		var resultBytes []byte

		if strings.Contains(originContents, "startTime") || strings.Contains(originContents, "error") {
			resultBytes = resp
		} else if annotResp.StatusCode == 204 {
			// A 204 status code indicates a successful delete with no content.
			status = "SUCCESS"
			message = fmt.Sprint("Successful delete with no content")

			return nil
		}

		err := json.Unmarshal(resultBytes, &annotResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal annotation response error: %s", err.Error())
			annotResp.logger.Errorf(message)

			return err
		}
		status = "SUCCESS"
		message = fmt.Sprint("annotation custom parsing successful")

		return nil
	}

}

// String returns the JSON representation of the AnnotationResponse as a string.
func (annotResp *AnnotationResponse) String(ctx context.Context) string {
	_, span := annotResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(annotResp.logger, time.Now(), "GetCustomParser-AnnotationResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(annotResp)
	if err != nil {
		message = fmt.Sprintf("marshal annotation response error: %s", err.Error())
		annotResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("annotation response converted to string successfully")

	return buffer.String()
}

// QueryAnnotation sends a GET request to query an annotation based on the provided parameters.
func (c *OpentsdbClient) QueryAnnotation(queryAnnoParam map[string]interface{}) (*AnnotationResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "QueryAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "QueryAnnotation", &status, &message, span)

	if queryAnnoParam == nil || len(queryAnnoParam) == 0 {
		message = "annotation query parameter is empty"
		return nil, errors.New(message)
	}

	buffer := bytes.NewBuffer(nil)

	size := len(queryAnnoParam)

	i := 0

	for k, v := range queryAnnoParam {
		buffer.WriteString(fmt.Sprintf("%s=%v", k, v))
		if i < size-1 {
			buffer.WriteString("&")
		}
		i++
	}

	annoEndpoint := fmt.Sprintf("%s%s?%s", c.tsdbEndpoint, AnnotationPath, buffer.String())
	annResp := AnnotationResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, annoEndpoint, "", &annResp); err != nil {
		message = fmt.Sprintf("error while processing annotation query: %s", err.Error())
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("Annotation query sent to url: %s", annoEndpoint)
	c.logger.Logf("Annotation query processed successfully")

	return &annResp, nil
}

// UpdateAnnotation sends a POST request to update an existing annotation.
func (c *OpentsdbClient) UpdateAnnotation(annotation Annotation) (*AnnotationResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "UpdateAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "UpdateAnnotation", &status, &message, span)

	annresp, err := c.operateAnnotation(PostMethod, &annotation)
	if err == nil {
		status = "SUCCESS"
		message = fmt.Sprintf("annotation with tsuid: %s updated successfully", annotation.Tsuid)
		c.logger.Logf("annotation updated successfully")
	}

	c.logger.Errorf("error while updating annotation")
	message = fmt.Sprintf("error while updating annotation with tsuid: %s", annotation.Tsuid)

	return annresp, err
}

// DeleteAnnotation sends a DELETE request to remove an existing annotation.
func (c *OpentsdbClient) DeleteAnnotation(annotation Annotation) (*AnnotationResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "DeleteAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "DeleteAnnotation", &status, &message, span)

	annresp, err := c.operateAnnotation(DeleteMethod, &annotation)
	if err == nil {
		status = "SUCCESS"
		message = fmt.Sprintf("annotation with tsuid: %s deleted successfully", annotation.Tsuid)
		c.logger.Logf("annotation deleted successfully")
	}

	c.logger.Errorf("error while deleting annotation")
	message = fmt.Sprintf("error while deleting annotation with tsuid: %s", annotation.Tsuid)

	return annresp, err
}

// operateAnnotation is a helper function to handle annotation operations (POST, DELETE).
func (c *OpentsdbClient) operateAnnotation(method string, annotation *Annotation) (*AnnotationResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "operateAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "operateAnnotation", &status, &message, span)

	if !c.isValidOperateMethod(method) {
		message = fmt.Sprintf("invalid annotation operation method: %s", method)
		return nil, errors.New(message)
	}

	annoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AnnotationPath)

	resultBytes, err := json.Marshal(annotation)
	if err != nil {
		message = fmt.Sprintf("marshal annotation response error: %s", err.Error())
		return nil, fmt.Errorf(message)
	}

	annResp := AnnotationResponse{logger: c.logger, tracer: c.tracer}

	if err = c.sendRequest(method, annoEndpoint, string(resultBytes), &annResp); err != nil {
		message = fmt.Sprintf("error while processing %s annotation request to url %s: %s", method, annoEndpoint, err.Error())
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("%s annotation request to url processed successfully", annoEndpoint)
	c.logger.Logf("%s annotation request successful", method)

	annResp.logger = c.logger
	annResp.tracer = c.tracer

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

// SetStatus sets the HTTP status code in the BulkAnnotatResponse.
func (bulkAnnotResp *BulkAnnotatResponse) SetStatus(ctx context.Context, code int) {
	_, span := bulkAnnotResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(bulkAnnotResp.logger, time.Now(), "SetStatus-BulkAnnotationResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	bulkAnnotResp.StatusCode = code
}

// GetCustomParser returns a custom parser function to handle the response from bulk annotation operations.
func (bulkAnnotResp *BulkAnnotatResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := bulkAnnotResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(bulkAnnotResp.logger, time.Now(), "GetCustomParser-BulkAnnotationResp", &status, &message, span)

	return func(respCnt []byte) error {
		originContents := string(respCnt)
		var resultBytes []byte

		if strings.Contains(originContents, "error") || strings.Contains(originContents, "totalDeleted") {
			resultBytes = respCnt
		} else if strings.Contains(originContents, "startTime") {
			resultBytes = []byte(fmt.Sprintf("{%s:%s}", `"InvolvedAnnotations"`, originContents))
		} else {
			message = fmt.Sprintf("Unrecognized bulk annotation response: %s", originContents)
			return errors.New(message)
		}

		err := json.Unmarshal(resultBytes, &bulkAnnotResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal bulk annotation response error: %s", err.Error())
			return err
		}

		status = "SUCCESS"
		message = fmt.Sprint("Custom parsing successful")

		return nil
	}
}

// String returns the JSON representation of the BulkAnnotatResponse as a string.
func (bulkAnnotResp *BulkAnnotatResponse) String(ctx context.Context) string {
	_, span := bulkAnnotResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(bulkAnnotResp.logger, time.Now(), "ToString-BulkAnnotationResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(bulkAnnotResp)
	if err != nil {
		message = fmt.Sprintf("marshal annotation bulk response error: %s", err.Error())
		bulkAnnotResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("annotation bulk response converted to json string successfully")

	return buffer.String()
}

// BulkUpdateAnnotations sends a POST request to update multiple annotations in bulk.
func (c *OpentsdbClient) BulkUpdateAnnotations(annotations []Annotation) (*BulkAnnotatResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "operateAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "BulkUpdateAnnotations", &status, &message, span)

	if annotations == nil || len(annotations) == 0 {
		message = fmt.Sprint("The annotations list is empty.")
		return nil, errors.New(message)
	}

	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)
	reqBodyCnt, err := marshalAnnotations(annotations)
	if err != nil {
		message = fmt.Sprintf("Failed to marshal annotations: %v", err)
		return nil, fmt.Errorf(message)
	}

	bulkAnnoResp := BulkAnnotatResponse{logger: c.logger, tracer: c.tracer}
	if err = c.sendRequest(PostMethod, bulkAnnoEndpoint, reqBodyCnt, &bulkAnnoResp); err != nil {
		message = fmt.Sprintf("error while processing update bulk annotations request to url %s: %s", bulkAnnoEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("Bulk annotation updation processed successfully at url %s", bulkAnnoEndpoint)
	c.logger.Logf("Bulk annotation updated successfully")
	return &bulkAnnoResp, nil
}

// BulkDeleteAnnotations sends a DELETE request to remove multiple annotations in bulk.
func (c *OpentsdbClient) BulkDeleteAnnotations(bulkDelParam BulkAnnoDeleteInfo) (*BulkAnnotatResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "operateAnnotation")

	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "BulkUpdateAnnotations", &status, &message, span)

	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)
	resultBytes, err := json.Marshal(bulkDelParam)

	if err != nil {
		message = fmt.Sprintf("failed to marshal bulk delete request parameters: %v", err)
		return nil, fmt.Errorf(message)
	}

	bulkAnnoResp := BulkAnnotatResponse{logger: c.logger, tracer: c.tracer}
	if err = c.sendRequest(DeleteMethod, bulkAnnoEndpoint, string(resultBytes), &bulkAnnoResp); err != nil {
		message = fmt.Sprintf("Bulk annotation delete request failed at url %s: %v", bulkAnnoEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprintf("Bulk annotations deleted successfully at url %s", bulkAnnoEndpoint)
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
