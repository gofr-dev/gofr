package http

import (
	"encoding/json"
	"net/http"
	"reflect"

	resTypes "gofr.dev/pkg/gofr/http/response"
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
	statusCode, errorObj := getStatusCode(r.method, data, err)

	var resp any
	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.Response:
		resp = response{Data: v.Data, Metadata: v.Metadata, Error: errorObj}
	case resTypes.File:
		r.w.Header().Set("Content-Type", v.ContentType)
		r.w.WriteHeader(statusCode)

		_, _ = r.w.Write(v.Content)

		return
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

func createErrorResponse(err error) map[string]any {
	return map[string]any{
		"message": err.Error(),
	}
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
