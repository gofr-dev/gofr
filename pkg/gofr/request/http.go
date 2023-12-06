package request

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"

	"gofr.dev/pkg/middleware/oauth"
)

// HTTP represents a utility for accessing and performing HTTP operations, such as reading an HTTP request
// and gathering information related to the request, including path parameters.
//
// The `HTTP` type provides a convenient interface for working with HTTP requests, extracting data from them,
// and processing related information. It is particularly useful for building applications that interact
// with the HTTP protocol and need to extract data from incoming requests.
type HTTP struct {
	req        *http.Request
	pathParams map[string]string
}

// NewHTTPRequest injects a *http.Request into a gofr.Request variable
func NewHTTPRequest(r *http.Request) Request {
	return &HTTP{
		req: r,
	}
}

// Request returns the underlying HTTP request
func (h *HTTP) Request() *http.Request {
	return h.req
}

// String satisfies the Stringer interface for the HTTP type
func (h *HTTP) String() string {
	return fmt.Sprintf("%s %s", h.req.Method, h.req.URL)
}

// Method returns the HTTP Method of the current request
func (h *HTTP) Method() string {
	return h.req.Method
}

// URI returns the current HTTP requests URL-PATH
func (h *HTTP) URI() string {
	return h.req.URL.Path
}

// Param returns the query parameter value for the given key, if any
func (h *HTTP) Param(key string) string {
	values := h.req.URL.Query()[key]
	return strings.Join(values, ",")
}

// ParamNames returns the list of query parameters (keys) for the current request
func (h *HTTP) ParamNames() []string {
	var ( //nolint:prealloc // Preallocating fails a testcase.
		names []string
		q     = h.req.URL.Query()
	)

	for name := range q {
		names = append(names, name)
	}

	return names
}

// Params returns the query parameters for the current request in the form of a mapping of key to it's values as
// comma separated values
func (h *HTTP) Params() map[string]string {
	res := make(map[string]string)
	for key, values := range h.req.URL.Query() {
		res[key] = strings.Join(values, ",")
	}

	return res
}

// PathParam returns the route values for the given key, if any
func (h *HTTP) PathParam(key string) string {
	if h.pathParams == nil {
		h.pathParams = make(map[string]string)
		h.pathParams = mux.Vars(h.req)
	}

	return h.pathParams[key]
}

// Body returns the request body, throws an error if there was one
func (h *HTTP) Body() ([]byte, error) {
	bodyBytes, err := io.ReadAll(h.req.Body)
	if err != nil {
		return nil, err
	}

	h.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}

// Header returns the value associated with the `key`, from the request headers
func (h *HTTP) Header(key string) string {
	return h.req.Header.Get(key)
}

// Bind checks the Content-Type to select a binding encoding automatically.
// Depending on the "Content-Type" header different bindings are used:
// - XML binding is used in case of: "application/xml" or "text/xml"
// - JSON binding is used by default
// It decodes the json payload into the type specified as a pointer.
// It returns an error if the decoding fails.
func (h *HTTP) Bind(i interface{}) error {
	body, err := h.Body()
	if err != nil {
		return err
	}

	cType := h.req.Header.Get("Content-type")

	switch {
	case strings.HasPrefix(cType, "text/xml"), strings.HasPrefix(cType, "application/xml"):
		return xml.Unmarshal(body, &i)
	case strings.HasPrefix(cType, "multipart/form-data"):
		if err := h.req.ParseMultipartForm(0); err != nil {
			return err
		}

		form := h.req.Form
		data := make(map[string]interface{})

		for key, values := range form {
			data[key] = values[0]
		}

		return mapstructure.Decode(data, &i)
	default:
		return json.Unmarshal(body, &i)
	}
}

// BindStrict checks the "Content-Type" header to select a binding encoding automatically.
// Depending on the "Content-Type" header, different bindings are used:
// - XML binding is used in case of "application/xml" or "text/xml" content type.
// - JSON binding is used by default.
// It decodes the JSON or XML payload into the type specified as a pointer.
// It returns an error if the decoding fails, and it disallows unknown fields
// when decoding JSON payloads to enforce strict parsing.
func (h *HTTP) BindStrict(i interface{}) error {
	body, err := h.Body()
	if err != nil {
		return err
	}

	cType := h.req.Header.Get("Content-type")
	switch cType {
	case "text/xml", "application/xml":
		return xml.Unmarshal(body, &i)
	default:
		dec := json.NewDecoder(h.req.Body)
		dec.DisallowUnknownFields()

		return dec.Decode(&i)
	}
}

// GetClaims function returns the map of claims
func (h *HTTP) GetClaims() map[string]interface{} {
	claims, ok := h.req.Context().Value(oauth.JWTContextKey("claims")).(jwt.MapClaims)
	if !ok {
		return nil
	}

	return claims
}

// GetClaim function returns the value of claim key provided as the parameter
func (h *HTTP) GetClaim(claimKey string) interface{} {
	claims := h.GetClaims()

	val, ok := claims[claimKey]
	if !ok {
		return nil
	}

	return val
}
