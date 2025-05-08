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
	var resp any

	switch v := data.(type) {
	case resTypes.File:
		r.w.Header().Set("Content-Type", v.ContentType)
		r.w.WriteHeader(http.StatusOK)

		_, _ = r.w.Write(v.Content)

		return
	case resTypes.Template:
		r.w.Header().Set("Content-Type", "text/html")
		v.Render(r.w)

		return
	case resTypes.Redirect:
		// HTTP 302 by default
		statusCode := http.StatusFound

		switch r.method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			// HTTP 303
			statusCode = http.StatusSeeOther
		}

		r.w.Header().Set("Location", v.URL)
		r.w.WriteHeader(statusCode)

		return
	}

	statusCode, errorObj := r.determineResponse(data, err)

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

	r.w.Header().Set("Content-Type", "application/json")

	r.w.WriteHeader(statusCode)

	_ = json.NewEncoder(r.w).Encode(resp)
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

	if e, ok := err.(statusCodeResponder); ok {
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

type statusCodeResponder interface {
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
