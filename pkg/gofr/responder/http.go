// Package responder provides an HTTP response handler for Go applications. It allows generating responses in different
// formats (JSON, XML, or plain text) based on context and data types. Additionally, it sets headers and handles
// response codes for various HTTP methods and errors.
package responder

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/template"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/middleware"
)

type responseType int

const (
	JSON responseType = iota
	XML
	TEXT
)

type HTTP struct {
	path          string
	method        string
	w             http.ResponseWriter
	resType       responseType
	correlationID string
}

// NewContextualResponder creates an HTTP responder which gives JSON/XML response based on context
func NewContextualResponder(w http.ResponseWriter, r *http.Request) Responder {
	route := mux.CurrentRoute(r)

	var path string
	if route != nil {
		path, _ = route.GetPathTemplate()
		// remove the trailing slash
		path = strings.TrimSuffix(path, "/")
	}

	var correlationID string

	val := r.Context().Value(middleware.CorrelationIDKey)
	if val != nil {
		correlationID = val.(string)
	}

	responder := &HTTP{
		w:             w,
		method:        r.Method,
		path:          path,
		correlationID: correlationID,
	}

	cType := r.Header.Get("Content-type")
	switch cType {
	case "text/xml", "application/xml":
		responder.resType = XML
	case "text/plain":
		responder.resType = TEXT
	default:
		responder.resType = JSON
	}

	return responder
}

// Respond generates an HTTP response based on the provided data and error.
// It sets the "X-Correlation-ID" header in the response and handles different response data types.
func (h HTTP) Respond(data interface{}, err error) {
	// set correlation id in response
	h.w.Header().Set("X-Correlation-ID", h.correlationID)

	// if template is returned then everything is dictated by template
	if d, ok := data.(template.Template); ok {
		var b []byte
		b, err = d.Render()

		if err != nil {
			h.processTemplateError(err)
			return
		}

		h.w.Header().Set("Content-Type", d.ContentType())
		h.w.WriteHeader(http.StatusOK)
		_, _ = h.w.Write(b)

		return
	}

	if f, ok := data.(template.File); ok {
		h.w.Header().Set("Content-Type", f.ContentType)
		_, _ = h.w.Write(f.Content)

		return
	}

	var (
		response   interface{}
		statusCode int
	)

	res, okay := data.(*types.Response)
	if res == nil {
		res = &types.Response{}
	}

	if !okay {
		response = data
		statusCode = getStatusCode(h.method, data, err)
	} else {
		response = getResponse(res, err)
		statusCode = getStatusCode(h.method, res.Data, err)
	}
	// This will check if data has the types.RawWithOptions type,
	// if true it will assign its Data to response and ContentType to h.resType and Header will be set.
	if tempData, ok := data.(types.RawWithOptions); ok {
		response = tempData.Data
		h.resType = getResponseContentType(tempData.ContentType, h.resType)
		setHeaders(tempData.Header, h.w)
	}

	h.processResponse(statusCode, response)
}

// setHeaders will set the value of header.
// If the header given is content-type or x-correlation-id it will not set that
func setHeaders(headers map[string]string, w http.ResponseWriter) {
	exemptedHeader := map[string]struct{}{
		"content-type":     {},
		"x-correlation-id": {},
	}

	for key, value := range headers {
		if _, ok := exemptedHeader[strings.ToLower(key)]; ok {
			continue
		}

		w.Header().Set(key, value)
	}
}

func (h HTTP) processResponse(statusCode int, response interface{}) {
	switch h.resType {
	case JSON:
		h.w.Header().Set("Content-type", "application/json")
		h.w.WriteHeader(statusCode)

		if response != nil {
			_ = json.NewEncoder(h.w).Encode(response)
		}

	case XML:
		h.w.Header().Set("Content-type", "application/xml")
		h.w.WriteHeader(statusCode)

		if response != nil {
			_ = xml.NewEncoder(h.w).Encode(response)
		}
	case TEXT:
		h.w.Header().Set("Content-type", "text/plain")
		h.w.WriteHeader(statusCode)

		if response != nil {
			_, _ = fmt.Fprintf(h.w, "%s", response)
		}
	}
}

func getStatusCode(method string, data interface{}, err error) int {
	statusCode := 200

	if err == nil {
		if method == http.MethodPost {
			statusCode = 201
		} else if method == http.MethodDelete {
			statusCode = 204
		}

		return statusCode
	}

	if e, ok := err.(errors.MultipleErrors); ok {
		if data != nil {
			return http.StatusPartialContent
		}

		statusCode = e.StatusCode
		if e.StatusCode == 0 {
			statusCode = http.StatusInternalServerError
		}

		return statusCode
	}

	return statusCode
}

func getResponse(res *types.Response, err error) interface{} {
	// Response error should be of MultipleErrors type
	em, ok := err.(errors.MultipleErrors)

	if res == nil || !ok {
		return res
	}

	if rawErr := checkRawErrorInMultipleErrors(em); rawErr != nil {
		return rawErr
	}

	// If data and error both are present (Partial Content)
	if res.Data != nil {
		dataMap := make(map[string]interface{})
		dataMap["errors"] = em.Errors

		b := new(bytes.Buffer)
		_ = json.NewEncoder(b).Encode(res.Data)
		_ = json.NewDecoder(b).Decode(&dataMap)

		// To handle the case of interface having nullable type and its value is nil
		if dataMap == nil {
			res.Data = nil // Ensuring response is not partial content
			return em
		}

		return types.Response{Data: dataMap, Meta: res.Meta}
	}

	// error is present but only status code is needed to be set and no body
	if em.Error() == "" {
		return nil
	}
	// error is set and returned in the body
	return em
}

func (h HTTP) processTemplateError(err error) {
	errorData := &errors.Response{}
	errorData.Reason = err.Error()

	switch err.(type) {
	case errors.FileNotFound:
		errorData.Code = "File Not Found"
		errorData.StatusCode = http.StatusNotFound
	default:
		errorData.StatusCode = http.StatusInternalServerError
		errorData.Code = "Internal Server Error"
		// pushing error type to prometheus
		middleware.ErrorTypesStats.With(prometheus.Labels{"type": "UnknownError", "path": h.path, "method": h.method}).Inc()
	}

	errMultiple := errors.MultipleErrors{
		StatusCode: errorData.StatusCode,
		Errors:     []error{errorData},
	}

	h.processResponse(errMultiple.StatusCode, errMultiple)
}

func checkRawErrorInMultipleErrors(err errors.MultipleErrors) error {
	if len(err.Errors) == 1 {
		if rem, ok := err.Errors[0].(errors.Raw); ok {
			if rem.Err == nil {
				return errors.Error("Unknown Error")
			}

			return rem.Err
		}
	}

	return nil
}

// getResponseContentType determines the response content type based on the provided input string.
func getResponseContentType(ctype string, defaults responseType) responseType {
	switch ctype {
	case "text/xml", "application/xml":
		return XML
	case "text/plain":
		return TEXT
	case "application/json":
		return JSON
	default:
		return defaults
	}
}
