package http

import (
	"encoding/json"
	"net/http"
)

func NewResponder(w http.ResponseWriter) *Responder {
	return &Responder{w: w}
}

type Responder struct {
	w http.ResponseWriter
}

func (r Responder) Respond(data interface{}, err error) {
	r.w.Header().Set("Content-type", "application/json")

	statusCode := r.HTTPStatusFromError(err)
	r.w.WriteHeader(statusCode)

	response := response{
		Error: err,
		Data:  data,
	}

	_ = json.NewEncoder(r.w).Encode(response)
}

func (r Responder) HTTPStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	return http.StatusInternalServerError
}

type response struct {
	Error interface{} `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}
