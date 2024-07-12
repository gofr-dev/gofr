package http

import (
	"encoding/json"
	"net/http"

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
func (r Responder) Respond(data interface{}, err error) {
	statusCode, errorObj := getStatusCode(r.method, data, err)

	var resp interface{}
	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.File:
		r.w.Header().Set("Content-Type", v.ContentType)
		r.w.WriteHeader(statusCode)

		_, _ = r.w.Write(v.Content)

		return
	default:
		resp = response{
			Data:  v,
			Error: errorObj,
		}
	}

	r.w.Header().Set("Content-Type", "application/json")

	r.w.WriteHeader(statusCode)

	_ = json.NewEncoder(r.w).Encode(resp)
}

// getStatusCode returns corresponding HTTP status codes.
func getStatusCode(method string, data interface{}, err error) (status int, errObj interface{}) {
	if err == nil {
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

	e, ok := err.(statusCodeResponder)
	if ok {
		return e.StatusCode(), map[string]interface{}{
			"message": err.Error(),
		}
	}

	return http.StatusInternalServerError, map[string]interface{}{
		"message": err.Error(),
	}
}

// response represents an HTTP response.
type response struct {
	Error interface{} `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

type statusCodeResponder interface {
	StatusCode() int
}
