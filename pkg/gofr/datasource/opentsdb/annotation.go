package opentsdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
}

// SetStatus sets the HTTP status code in the AnnotationResponse.
func (annotResp *AnnotationResponse) SetStatus(code int) {
	annotResp.StatusCode = code
}

// GetCustomParser returns a custom parser function to process the response content
// from an annotation-related API request.
func (annotResp *AnnotationResponse) GetCustomParser() func(resp []byte) error {
	return func(resp []byte) error {
		originContents := string(resp)
		var resultBytes []byte
		if strings.Contains(originContents, "startTime") || strings.Contains(originContents, "error") {
			resultBytes = resp
		} else if annotResp.StatusCode == 204 {
			// A 204 status code indicates a successful delete with no content.
			return nil
		}
		return json.Unmarshal(resultBytes, &annotResp)
	}
}

// String returns the JSON representation of the AnnotationResponse as a string.
func (annotResp *AnnotationResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(annotResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

// QueryAnnotation sends a GET request to query an annotation based on the provided parameters.
func (c *OpentsdbClient) QueryAnnotation(queryAnnoParam map[string]interface{}) (*AnnotationResponse, error) {
	if queryAnnoParam == nil || len(queryAnnoParam) == 0 {
		return nil, errors.New("the query annotation parameter is nil")
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
	annResp := AnnotationResponse{}
	if err := c.sendRequest(GetMethod, annoEndpoint, "", &annResp); err != nil {
		return nil, err
	}
	return &annResp, nil
}

// UpdateAnnotation sends a POST request to update an existing annotation.
func (c *OpentsdbClient) UpdateAnnotation(annotation Annotation) (*AnnotationResponse, error) {
	return c.operateAnnotation(PostMethod, &annotation)
}

// DeleteAnnotation sends a DELETE request to remove an existing annotation.
func (c *OpentsdbClient) DeleteAnnotation(annotation Annotation) (*AnnotationResponse, error) {
	return c.operateAnnotation(DeleteMethod, &annotation)
}

// operateAnnotation is a helper function to handle annotation operations (POST, DELETE).
func (c *OpentsdbClient) operateAnnotation(method string, annotation *Annotation) (*AnnotationResponse, error) {
	if !c.isValidOperateMethod(method) {
		return nil, errors.New("The method for operating an annotation is invalid.")
	}
	annoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AnnotationPath)
	resultBytes, err := json.Marshal(annotation)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal annotation: %v", err)
	}
	annResp := AnnotationResponse{}
	if err = c.sendRequest(method, annoEndpoint, string(resultBytes), &annResp); err != nil {
		return nil, err
	}
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

	// BulkDeleteResp holds details about the bulk delete operation, if applicable.
	BulkDeleteResp
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

	// TotalDeleted holds the total number of annotations successfully deleted in the bulk operation.
	TotalDeleted int64 `json:"totalDeleted,omitempty"`
}

// SetStatus sets the HTTP status code in the BulkAnnotatResponse.
func (bulkAnnotResp *BulkAnnotatResponse) SetStatus(code int) {
	bulkAnnotResp.StatusCode = code
}

// GetCustomParser returns a custom parser function to handle the response from bulk annotation operations.
func (bulkAnnotResp *BulkAnnotatResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		originContents := string(respCnt)
		var resultBytes []byte
		if strings.Contains(originContents, "startTime") {
			resultBytes = []byte(fmt.Sprintf("{%s:%s}", `"InvolvedAnnotations"`, originContents))
		} else if strings.Contains(originContents, "error") || strings.Contains(originContents, "totalDeleted") {
			resultBytes = respCnt
		} else {
			return errors.New(fmt.Sprintf("Unrecognized bulk annotation response: %s", originContents))
		}
		return json.Unmarshal(resultBytes, &bulkAnnotResp)
	}
}

// String returns the JSON representation of the BulkAnnotatResponse as a string.
func (bulkAnnotResp *BulkAnnotatResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(bulkAnnotResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

// BulkUpdateAnnotations sends a POST request to update multiple annotations in bulk.
func (c *OpentsdbClient) BulkUpdateAnnotations(annotations []Annotation) (*BulkAnnotatResponse, error) {
	if annotations == nil || len(annotations) == 0 {
		return nil, errors.New("The annotations list is empty.")
	}
	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)
	reqBodyCnt, err := marshalAnnotations(annotations)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal annotations: %v", err)
	}
	bulkAnnoResp := BulkAnnotatResponse{}
	if err = c.sendRequest(PostMethod, bulkAnnoEndpoint, reqBodyCnt, &bulkAnnoResp); err != nil {
		return nil, err
	}
	return &bulkAnnoResp, nil
}

// BulkDeleteAnnotations sends a DELETE request to remove multiple annotations in bulk.
func (c *OpentsdbClient) BulkDeleteAnnotations(bulkDelParam BulkAnnoDeleteInfo) (*BulkAnnotatResponse, error) {
	bulkAnnoEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, BulkAnnotationPath)
	resultBytes, err := json.Marshal(bulkDelParam)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal bulk delete parameters: %v", err)
	}
	bulkAnnoResp := BulkAnnotatResponse{}
	if err = c.sendRequest(DeleteMethod, bulkAnnoEndpoint, string(resultBytes), &bulkAnnoResp); err != nil {
		return nil, err
	}
	return &bulkAnnoResp, nil
}

// marshalAnnotations converts a slice of annotations into a JSON string for bulk operations.
func marshalAnnotations(annotations []Annotation) (string, error) {
	resultBytes, err := json.Marshal(annotations)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal annotations: %v", err)
	}
	return string(resultBytes), nil
}
