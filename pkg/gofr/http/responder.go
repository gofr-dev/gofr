package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

// NewResponder creates a new Responder instance from the given http.ResponseWriter..
func NewResponder(w http.ResponseWriter, method string) *Responder {
	return &Responder{w: w, method: method}
}

// Responder encapsulates a http.ResponseWriter and is responsible for crafting structured responses.
type Responder struct {
	w      http.ResponseWriter
	method string
}

// response represents an HTTP response.
type response struct {
	Data   interface{}   `json:"data,omitempty"`
	Errors []errResponse `json:"errors,omitempty"`
}

type errResponse struct {
	Reason   string      `json:"reason"`
	Details  interface{} `json:"details,omitempty"`
	DateTime time.Time   `json:"datetime"`
}

type statusCodeResponder interface {
	StatusCode() int
	Error() string
}

// Respond sends a response with the given data and handles potential errors, setting appropriate
// status codes and formatting responses as JSON or raw data as needed.
func (r Responder) Respond(data interface{}, err error) {
	statusCode := getStatusCode(r.method, data, err)
	errObj := getErrResponse(err)

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
			Data:   v,
			Errors: errObj,
		}
	}

	r.w.Header().Set("Content-Type", "application/json")

	r.w.WriteHeader(statusCode)

	_ = json.NewEncoder(r.w).Encode(resp)
}

// getStatusCode returns corresponding HTTP status codes.
func getStatusCode(method string, data interface{}, err error) (status int) {
	if err == nil {
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

	var e statusCodeResponder
	if errors.As(err, &e) {
		if data != nil {
			return http.StatusPartialContent
		}

		status = e.StatusCode()

		if e.StatusCode() == 0 {
			return http.StatusInternalServerError
		}

		return status
	}

	return http.StatusInternalServerError
}

func getErrResponse(err error) []errResponse {
	var (
		errResp []errResponse
		m       MultipleErrors
		c       CustomError
	)

	if err != nil {
		switch {
		case errors.As(err, &m):
			for _, v := range m.Errors {
				resp := errResponse{Reason: v.Error(), DateTime: time.Now()}

				if errors.As(v, &c) {
					resp.Details = c.Details
				}

				errResp = append(errResp, resp)
			}
		case errors.As(err, &c):
			errResp = append(errResp, errResponse{Reason: c.Reason, Details: c.Details, DateTime: time.Now()})
		default:
			errResp = append(errResp, errResponse{Reason: err.Error(), DateTime: time.Now()})
		}

		return errResp
	}

	return nil
}
