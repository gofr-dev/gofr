package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

var (
	errEmptyResponse = errors.New("internal server error")
)

// NewResponder creates a new Responder instance from the given http.ResponseWriter..
func NewResponder(w http.ResponseWriter, method string) *Responder {
	return &Responder{w: w, method: method}
}

// Responder encapsulates an http.ResponseWriter and is responsible for crafting structured responses.
type Responder struct {
	w      http.ResponseWriter
	method string
}

// Respond sends a response with the given data and handles potential errors, setting appropriate
// status codes and formatting responses as JSON or raw data as needed.
func (r Responder) Respond(data any, err error) {
	if r.handleSpecialResponseTypes(data, err) {
		return
	}

	statusCode, errorObj := r.determineResponse(data, err)

	var resp any

	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.Response:
		resp = response{Data: v.Data, Metadata: v.Metadata, Error: errorObj}
	default:
		// handling where an interface contains a nullable type with a nil value.
		if isNil(data) {
			data = nil
		}

		resp = response{Data: data, Error: errorObj}
	}

	if r.w.Header().Get("Content-Type") == "" {
		r.w.Header().Set("Content-Type", "application/json")
	}

	jsonData, encodeErr := json.Marshal(resp)
	if encodeErr != nil {
		r.w.WriteHeader(http.StatusInternalServerError)

		errorResp := response{Error: map[string]any{"message": "failed to encode response as JSON"}}
		errorJSON, _ := json.Marshal(errorResp)

		_, _ = r.w.Write(errorJSON)
		_, _ = r.w.Write([]byte("\n"))

		return
	}

	r.w.WriteHeader(statusCode)
	_, _ = r.w.Write(jsonData)
	_, _ = r.w.Write([]byte("\n"))
}

// handleSpecialResponseTypes handles special response types that bypass JSON encoding.
// Returns true if the response was handled, false otherwise.
func (r Responder) handleSpecialResponseTypes(data any, err error) bool {
	// For special response types (XML/File/Template), use error status code directly
	// instead of partial content (206) when errors occur
	statusCode := r.getStatusCodeForSpecialResponse(data, err)

	switch v := data.(type) {
	case resTypes.File:
		r.w.Header().Set("Content-Type", v.ContentType)
		r.w.WriteHeader(statusCode)
		_, _ = r.w.Write(v.Content)

		return true

	case resTypes.Template:
		r.w.Header().Set("Content-Type", "text/html")
		r.w.WriteHeader(statusCode)
		v.Render(r.w)

		return true

	case resTypes.XML:
		contentType := v.ContentType

		if contentType == "" {
			contentType = "application/xml"
		}

		r.w.Header().Set("Content-Type", contentType)
		r.w.WriteHeader(statusCode)

		if len(v.Content) > 0 {
			_, _ = r.w.Write(v.Content)
		}

		return true

	case resTypes.Redirect:
		// Redirect status codes are determined by HTTP method, not error state
		redirectStatusCode := http.StatusFound

		if r.method == http.MethodPost || r.method == http.MethodPut || r.method == http.MethodPatch {
			redirectStatusCode = http.StatusSeeOther
		}

		r.w.Header().Set("Location", v.URL)
		r.w.WriteHeader(redirectStatusCode)

		return true
	}

	return false
}

// getStatusCodeForSpecialResponse returns the appropriate status code for special response types.
// Unlike regular responses, special types (XML/File/Template) should use error status codes
// directly instead of returning 206 (Partial Content) when both data and error are present.
func (r Responder) getStatusCodeForSpecialResponse(data any, err error) int {
	if err == nil {
		return handleSuccessStatusCode(r.method, data)
	}

	// For special response types, prioritize error status code over partial content
	if e, ok := err.(StatusCodeResponder); ok {
		return e.StatusCode()
	}

	return http.StatusInternalServerError
}

// handleSuccessStatusCode returns the status code for successful responses based on HTTP method.
func handleSuccessStatusCode(method string, data any) int {
	switch method {
	case http.MethodPost:
		if data != nil {
			return http.StatusCreated
		}

		return http.StatusAccepted
	case http.MethodDelete:
		return http.StatusNoContent
	default:
		return http.StatusOK
	}
}

func (r Responder) determineResponse(data any, err error) (statusCode int, errObj any) {
	// Handle empty struct case first
	if err != nil && isEmptyStruct(data) {
		return http.StatusInternalServerError, createErrorResponse(errEmptyResponse)
	}

	statusCode, errorObj := getStatusCode(r.method, data, err)

	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	return statusCode, errorObj
}

// isEmptyStruct checks if a value is a struct with all zero/empty fields.
func isEmptyStruct(data any) bool {
	if data == nil {
		return false
	}

	v := reflect.ValueOf(data)

	// Handle pointers by dereferencing them
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false // nil pointer isn't an empty struct
		}

		v = v.Elem()
	}

	// Only check actual struct types
	if v.Kind() != reflect.Struct {
		return false
	}

	// Compare against a zero value of the same type
	zero := reflect.Zero(v.Type()).Interface()

	return reflect.DeepEqual(data, zero)
}

// getStatusCode returns corresponding HTTP status codes.
func getStatusCode(method string, data any, err error) (statusCode int, errResp any) {
	if err == nil {
		return handleSuccess(method, data)
	}

	if !isNil(data) {
		return http.StatusPartialContent, createErrorResponse(err)
	}

	if e, ok := err.(StatusCodeResponder); ok {
		return e.StatusCode(), createErrorResponse(err)
	}

	return http.StatusInternalServerError, createErrorResponse(err)
}

func handleSuccess(method string, data any) (statusCode int, err any) {
	switch method {
	case http.MethodPost:
		if data != nil {
			return http.StatusCreated, nil
		}

		return http.StatusAccepted, nil
	case http.MethodDelete:
		return http.StatusNoContent, nil
	default:
		return http.StatusOK, nil
	}
}

// ResponseMarshaller defines an interface for errors that can provide custom fields.
// This enables errors to extend the error response with additional fields.
type ResponseMarshaller interface {
	Response() map[string]any
}

// createErrorResponse returns an error response that always contains a "message" field,
// and if the error implements ResponseMarshaller, it merges custom fields into the response.
func createErrorResponse(err error) map[string]any {
	resp := map[string]any{"message": err.Error()}

	if rm, ok := err.(ResponseMarshaller); ok {
		for k, v := range rm.Response() {
			if k == "message" {
				continue // Skip to avoid overriding the Error() message
			}

			resp[k] = v
		}
	}

	return resp
}

// response represents an HTTP response.
type response struct {
	Error    any            `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Data     any            `json:"data,omitempty"`
}

type StatusCodeResponder interface {
	StatusCode() int
}

// isNil checks if the given any value is nil.
// It returns true if the value is nil or if it is a pointer that points to nil.
// This function is useful for determining whether a value, including interface or pointer types, is effectively nil.
func isNil(i any) bool {
	if i == nil {
		return true
	}

	v := reflect.ValueOf(i)

	return v.Kind() == reflect.Ptr && v.IsNil()
}
