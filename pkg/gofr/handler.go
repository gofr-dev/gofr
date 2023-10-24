package gofr

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/template"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/middleware"
)

// Handler responds to HTTP request
// It takes in a custom Context type which holds all the information related to the incoming HTTP request
// It puts out an interface and error as output depending on what needs to be responded
// It
type Handler func(c *Context) (interface{}, error)

// ServeHTTP processes incoming HTTP requests. It extracts the request context, handles errors,
// determines appropriate responses based on the data type, and sends the response back to the client.
// The method dynamically handles various response formats, such as custom types, templates, and raw data.
func (h Handler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	c, _ := r.Context().Value(gofrContextkey).(*Context)

	data, err := h(c)

	route := mux.CurrentRoute(r)
	path, _ := route.GetPathTemplate()
	// remove the trailing slash
	path = strings.TrimSuffix(path, "/")

	var errorResp error

	if _, ok := err.(errors.EntityAlreadyExists); ok || err == nil {
		errorResp = err
	} else {
		isPartialResponse := data != nil // since err!=nil we can check if data is not nil
		errorResp = processErrors(err, path, r.Method, isPartialResponse)

		// set the error in the context, which can be fetched in the logging middleware
		ctx := context.WithValue(r.Context(), middleware.ErrorMessage, err.Error())
		*r = *r.Clone(ctx)
	}

	switch res := data.(type) {
	case types.Response:
		c.resp.Respond(&res, errorResp)
	case template.Template, template.File, *types.Response, types.RawWithOptions:
		c.resp.Respond(res, errorResp)
	case types.Raw:
		c.resp.Respond(res.Data, errorResp)
	default:
		res = &types.Response{Data: data}
		c.resp.Respond(res, errorResp)
	}
}

//nolint:gocognit,gocyclo // cannot be simplified further without hurting readability
func processErrors(err error, path, method string, isPartialError bool) errors.MultipleErrors {
	var errResp errors.Response

	errResp.Value, errResp.TimeZone = evaluateTimeAndTimeZone()
	errResp.Reason = err.Error()

	switch v := err.(type) {
	case errors.InvalidParam:
		errResp.StatusCode = http.StatusBadRequest
		errResp.Code = "Invalid Parameter"
	case errors.MissingParam:
		errResp.StatusCode = http.StatusBadRequest
		errResp.Code = "Missing Parameter"
	case errors.EntityNotFound:
		errResp.StatusCode = http.StatusNotFound
		errResp.Code = "Entity Not Found"
	case errors.FileNotFound:
		errResp.StatusCode = http.StatusNotFound
		errResp.Code = "File Not Found"
	case errors.MethodMissing:
		errResp.StatusCode = http.StatusMethodNotAllowed
		errResp.Code = "Method not allowed"
	case *errors.Response:
		if v.DateTime.Value == "" {
			v.DateTime = errResp.DateTime
		}
		// pushing error type to prometheus
		if (v.StatusCode == http.StatusInternalServerError || v.StatusCode == 0) && !isPartialError {
			middleware.ErrorTypesStats.With(prometheus.Labels{"type": "UnknownError", "path": path, "method": method}).Inc()
		}

		errResp = *v
	case errors.MultipleErrors:
		var finalErr errors.MultipleErrors
		finalErr.StatusCode = v.StatusCode
		now := time.Now()
		timeZone, _ := now.Zone()

		for _, v := range v.Errors {
			resp := errors.Response{}
			resp.TimeZone = timeZone
			resp.Value = now.UTC().Format(time.RFC3339)

			errs := processErrors(v, path, method, isPartialError)

			finalErr.Errors = append(finalErr.Errors, errs.Errors...)
		}

		return finalErr
	case errors.DB:
		errResp.StatusCode = http.StatusInternalServerError
		errResp.Code = "Internal Server Error"
		errResp.Reason = "DB Error"
		// pushing error type to prometheus
		middleware.ErrorTypesStats.With(prometheus.Labels{"type": "DBError", "path": path, "method": method}).Inc()
	case errors.Raw:
		return errors.MultipleErrors{StatusCode: v.StatusCode, Errors: []error{v}}
	default:
		errResp.StatusCode = http.StatusInternalServerError
		errResp.Code = "Internal Server Error"
		// pushing error type to prometheus
		if !isPartialError {
			middleware.ErrorTypesStats.With(prometheus.Labels{"type": "UnknownError", "path": path, "method": method}).Inc()
		}
	}

	return errors.MultipleErrors{StatusCode: errResp.StatusCode, Errors: []error{&errResp}}
}

func evaluateTimeAndTimeZone() (formattedTime, timeZone string) {
	now := time.Now()
	formattedTime = now.UTC().Format(time.RFC3339)
	timeZone, _ = now.Zone()

	return formattedTime, timeZone
}
