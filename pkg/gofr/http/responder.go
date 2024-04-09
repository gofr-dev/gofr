package http

import (
	"encoding/json"
	"errors"
	"net/http"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

// NewResponder creates a new Responder instance from the given http.ResponseWriter..
func NewResponder(w http.ResponseWriter) *Responder {
	return &Responder{w: w}
}

// Responder encapsulates an http.ResponseWriter and is responsible for crafting structured responses.
type Responder struct {
	w http.ResponseWriter
}

// Respond sends a response with the given data and handles potential errors, setting appropriate
// status codes and formatting responses as JSON or raw data as needed.
func (r Responder) Respond(data interface{}, err error) {
	statusCode, errorObj := r.HTTPStatusFromError(err)

	writeResponse(r.w, data, errorObj, statusCode)
}

// HTTPStatusFromError maps errors to HTTP status codes.
func (r Responder) HTTPStatusFromError(err error) (status int, errObj interface{}) {
	if err == nil {
		return http.StatusOK, nil
	}

	if errors.Is(err, http.ErrMissingFile) {
		return http.StatusNotFound, map[string]interface{}{
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

// NewPostResponder creates a new PostResponderResponder instance from the given http.ResponseWriter..
func NewPostResponder(w http.ResponseWriter) *PostResponder {
	return &PostResponder{w: w}
}

// PostResponder encapsulates an http.ResponseWriter and is responsible for crafting structured responses.
type PostResponder struct {
	w http.ResponseWriter
}

// Respond sends a response with the given data and handles potential errors, setting appropriate
// status codes and formatting responses as JSON or raw data as needed.
func (r PostResponder) Respond(data interface{}, err error) {
	statusCode, errorObj := r.HTTPStatusFromError(err)

	writeResponse(r.w, data, errorObj, statusCode)
}

// HTTPStatusFromError maps errors to HTTP status codes.
func (r PostResponder) HTTPStatusFromError(err error) (status int, errObj interface{}) {
	if err == nil {
		return http.StatusCreated, nil
	}

	if errors.Is(err, http.ErrMissingFile) {
		return http.StatusNotFound, map[string]interface{}{
			"message": err.Error(),
		}
	}

	return http.StatusInternalServerError, map[string]interface{}{
		"message": err.Error(),
	}
}

func writeResponse(w http.ResponseWriter, data, errorObj interface{}, statusCode int) {
	var resp interface{}
	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.File:
		w.Header().Set("Content-Type", v.ContentType)
		w.WriteHeader(statusCode)

		_, _ = w.Write(v.Content)

		return
	default:
		resp = response{
			Data:  v,
			Error: errorObj,
		}
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(resp)
}

// NewDeleteResponder creates a new PostResponderResponder instance from the given http.ResponseWriter..
func NewDeleteResponder(w http.ResponseWriter) *DeleteResponder {
	return &DeleteResponder{w: w}
}

// DeleteResponder encapsulates an http.ResponseWriter and is responsible for crafting structured responses.
type DeleteResponder struct {
	w http.ResponseWriter
}

// Respond sends a response with the given data and handles potential errors, setting appropriate
// status codes and formatting responses as JSON or raw data as needed.
func (r DeleteResponder) Respond(data interface{}, err error) {
	statusCode, errorObj := r.HTTPStatusFromError(err)

	writeResponse(r.w, data, errorObj, statusCode)
}

// HTTPStatusFromError maps errors to HTTP status codes.
func (r DeleteResponder) HTTPStatusFromError(err error) (status int, errObj interface{}) {
	if err == nil {
		return http.StatusNoContent, nil
	}

	if errors.Is(err, http.ErrMissingFile) {
		return http.StatusNotFound, map[string]interface{}{
			"message": err.Error(),
		}
	}

	return http.StatusInternalServerError, map[string]interface{}{
		"message": err.Error(),
	}
}
