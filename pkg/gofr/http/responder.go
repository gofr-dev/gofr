package http

import (
	"encoding/json"
	"net/http"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

func NewResponder(w http.ResponseWriter) *Responder {
	return &Responder{w: w}
}

type Responder struct {
	w http.ResponseWriter
}

func (r Responder) Respond(data interface{}, err error) {
	statusCode, errorObj := r.HTTPStatusFromError(err)
	r.w.WriteHeader(statusCode)

	var resp interface{}
	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.File:
		r.w.Header().Set("Content-Type", v.ContentType)
		_, _ = r.w.Write(v.Content)

		return
	default:
		resp = response{
			Data:  v,
			Error: errorObj,
		}
	}

	r.w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(r.w).Encode(resp)
}

func (r Responder) HTTPStatusFromError(err error) (status int, errObj interface{}) {
	if err == nil {
		return http.StatusOK, nil
	}

	return http.StatusInternalServerError, map[string]interface{}{
		"message": err.Error(),
	}
}

type response struct {
	Error interface{} `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}
